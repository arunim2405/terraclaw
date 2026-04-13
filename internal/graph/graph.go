// Package graph builds an in-memory resource dependency graph from Steampipe
// data. The graph detects relationships between cloud resources by
// cross-matching IDs, ARNs, and other reference properties, enabling users to
// select related resources together for better Terraform code generation.
//
// The architecture is provider-agnostic: the core Graph and Node types are
// generic, while relationship detection strategies can be plugged in per
// cloud provider.
package graph

import (
	"fmt"
	"strings"

	"github.com/arunim2405/terraclaw/internal/debuglog"
	"github.com/arunim2405/terraclaw/internal/provider"
	"github.com/arunim2405/terraclaw/internal/steampipe"
)

// ---------------------------------------------------------------------------
// Default AWS tables to scan when user picks "key resources" mode.
// These cover the most commonly cross-referenced resource types.
// ---------------------------------------------------------------------------

// DefaultAWSTables is the curated set of high-value AWS tables scanned by
// default in "key resources" mode, covering networking, compute, IAM, storage,
// and database layers.
var DefaultAWSTables = []string{
	// Networking
	"aws_vpc",
	"aws_subnet",
	"aws_security_group",
	"aws_internet_gateway",
	"aws_nat_gateway",
	"aws_route_table",
	"aws_network_interface",
	"aws_eip",

	// Compute
	"aws_ec2_instance",
	"aws_ec2_launch_template",
	"aws_autoscaling_group",
	"aws_lambda_function",
	"aws_ecs_cluster",
	"aws_ecs_service",
	"aws_ecs_task_definition",

	// IAM
	"aws_iam_role",
	"aws_iam_policy",
	"aws_iam_user",
	"aws_iam_group",
	"aws_iam_instance_profile",

	// Storage
	"aws_s3_bucket",
	"aws_ebs_volume",
	"aws_efs_file_system",

	// Databases
	"aws_rds_db_instance",
	"aws_rds_db_cluster",
	"aws_dynamodb_table",
	"aws_elasticache_cluster",

	// Auth / Identity
	"aws_cognito_user_pool",
	"aws_cognito_identity_pool",

	// Load Balancers
	"aws_ec2_application_load_balancer",
	"aws_ec2_network_load_balancer",
	"aws_ec2_target_group",

	// DNS
	"aws_route53_zone",

	// KMS
	"aws_kms_key",

	// SNS / SQS
	"aws_sns_topic",
	"aws_sqs_queue",

	// CloudWatch
	"aws_cloudwatch_log_group",
}

// DefaultAzureTables is the curated set of high-value Azure tables scanned by
// default in "key resources" mode, covering networking, compute, identity,
// storage, and database layers.
var DefaultAzureTables = []string{
	// Networking
	"azure_virtual_network",
	"azure_subnet",
	"azure_network_security_group",
	"azure_public_ip",
	"azure_lb",
	"azure_application_gateway",
	"azure_network_interface",
	"azure_route_table",
	"azure_nat_gateway",

	// Compute
	"azure_compute_virtual_machine",
	"azure_compute_virtual_machine_scale_set",
	"azure_compute_disk",
	"azure_compute_availability_set",

	// Containers
	"azure_kubernetes_cluster",
	"azure_container_registry",

	// App Services
	"azure_app_service_web_app",
	"azure_app_service_plan",
	"azure_app_service_function_app",

	// Identity & Access
	"azure_managed_identity",
	"azure_role_assignment",

	// Storage
	"azure_storage_account",

	// Databases
	"azure_sql_server",
	"azure_sql_database",
	"azure_postgresql_flexible_server",
	"azure_mysql_flexible_server",
	"azure_cosmosdb_account",
	"azure_redis_cache",

	// Key Vault
	"azure_key_vault",

	// Monitoring
	"azure_application_insights",
	"azure_log_analytics_workspace",

	// DNS
	"azure_dns_zone",
	"azure_private_dns_zone",

	// Messaging
	"azure_servicebus_namespace",
	"azure_eventhub_namespace",

	// Resource Management
	"azure_resource_group",
}

// DefaultTablesForProvider returns the default table list for the given cloud
// provider. Falls back to DefaultAWSTables for unrecognised providers.
func DefaultTablesForProvider(cloud provider.Cloud) []string {
	switch cloud {
	case provider.Azure:
		return DefaultAzureTables
	default:
		return DefaultAWSTables
	}
}

// ---------------------------------------------------------------------------
// Core types
// ---------------------------------------------------------------------------

// Node wraps a Steampipe resource and tracks its connections to other nodes.
type Node struct {
	// Key uniquely identifies this node: "<table>/<id>".
	Key string

	// Resource is the underlying Steampipe resource.
	Resource steampipe.Resource

	// Edges contains the keys of all directly related nodes.
	Edges map[string]bool
}

// AddEdge creates a bidirectional link between n and other.
func (n *Node) AddEdge(otherKey string) {
	if n.Edges == nil {
		n.Edges = make(map[string]bool)
	}
	n.Edges[otherKey] = true
}

// Graph holds all discovered resources and their relationships.
type Graph struct {
	// Nodes maps nodeKey → Node.
	Nodes map[string]*Node

	// idIndex maps raw ID/ARN values to the owning node key for fast lookups.
	idIndex map[string]string

	// Stats tracks scan progress.
	Stats ScanStats
}

// ScanStats records high-level metrics about the scan.
type ScanStats struct {
	TablesScanned int
	TotalTables   int
	ResourceCount int
	EdgeCount     int
}

// ProgressFunc is called during scanning to report progress.
// tablesScanned and totalTables indicate how far along we are.
type ProgressFunc func(tablesScanned, totalTables int, currentTable string)

// New creates an empty Graph.
func New() *Graph {
	return &Graph{
		Nodes:   make(map[string]*Node),
		idIndex: make(map[string]string),
	}
}

// AddNode inserts a resource into the graph. It is used by the cache layer
// to reconstruct a graph without a live Steampipe connection.
func (g *Graph) AddNode(r steampipe.Resource) {
	key := nodeKey(r.Type, r.ID)
	node := &Node{
		Key:      key,
		Resource: r,
		Edges:    make(map[string]bool),
	}
	g.Nodes[key] = node
	g.indexResource(key, r)
}

// AddEdge creates a bidirectional link between two node keys.
// It is exported so the cache layer can restore edges.
func (g *Graph) AddEdge(keyA, keyB string) {
	g.addEdge(keyA, keyB)
}

// ---------------------------------------------------------------------------
// Build: scan Steampipe tables and populate the graph
// ---------------------------------------------------------------------------

// Build scans the given tables in the provided schema using the Steampipe
// client and populates the graph with discovered resources. It calls
// progressFn (if non-nil) after each table is scanned.
func (g *Graph) Build(client *steampipe.Client, schema string, tables []string, progressFn ProgressFunc) error {
	g.Stats.TotalTables = len(tables)

	for i, table := range tables {
		if progressFn != nil {
			progressFn(i, len(tables), table)
		}

		resources, err := client.FetchResources(schema, table)
		if err != nil {
			// Log and skip tables that fail (e.g. permission issues).
			debuglog.Log("[graph] skipping table %s: %v", table, err)
			continue
		}

		for _, r := range resources {
			key := nodeKey(table, r.ID)
			node := &Node{
				Key:      key,
				Resource: r,
				Edges:    make(map[string]bool),
			}
			g.Nodes[key] = node

			// Index all identifiers for later relationship detection.
			g.indexResource(key, r)
		}

		g.Stats.TablesScanned = i + 1
		g.Stats.ResourceCount = len(g.Nodes)
		debuglog.Log("[graph] scanned %s: %d resource(s), total=%d", table, len(resources), len(g.Nodes))
	}

	if progressFn != nil {
		progressFn(len(tables), len(tables), "done")
	}

	return nil
}

// indexResource adds all plausible identifiers for a resource into the idIndex.
func (g *Graph) indexResource(key string, r steampipe.Resource) {
	// Index the primary ID and ARN.
	if r.ID != "" {
		g.idIndex[r.ID] = key
	}
	// Also index well-known identifier properties.
	for _, prop := range []string{"arn", "id", "akas"} {
		if v, ok := r.Properties[prop]; ok && v != "" {
			g.idIndex[v] = key
		}
	}
}

// ---------------------------------------------------------------------------
// DetectRelationships: cross-match IDs/ARNs in resource properties
// ---------------------------------------------------------------------------

// DetectRelationships scans every resource's properties and creates edges
// when a property value matches another resource's ID or ARN.
func (g *Graph) DetectRelationships() {
	edgeCount := 0
	for key, node := range g.Nodes {
		for propName, propValue := range node.Resource.Properties {
			if propValue == "" || len(propValue) > 500 {
				continue
			}

			// Check for direct match first (fastest path).
			if targetKey, ok := g.idIndex[propValue]; ok && targetKey != key {
				g.addEdge(key, targetKey)
				edgeCount++
				continue
			}

			// Check if the property name suggests a reference (_id, _arn suffixes).
			nameLower := strings.ToLower(propName)
			if !isReferenceProp(nameLower) {
				continue
			}

			// Scan for embedded IDs in array-like or complex values.
			for indexedID, targetKey := range g.idIndex {
				if targetKey == key {
					continue
				}
				if strings.Contains(propValue, indexedID) {
					g.addEdge(key, targetKey)
					edgeCount++
				}
			}
		}
	}
	g.Stats.EdgeCount = edgeCount
	debuglog.Log("[graph] detected %d relationship(s) across %d node(s)", edgeCount, len(g.Nodes))
}

// addEdge creates a bidirectional link between two nodes.
func (g *Graph) addEdge(keyA, keyB string) {
	if a, ok := g.Nodes[keyA]; ok {
		a.AddEdge(keyB)
	}
	if b, ok := g.Nodes[keyB]; ok {
		b.AddEdge(keyA)
	}
}

// isReferenceProp checks if a property name looks like a foreign-key reference.
func isReferenceProp(name string) bool {
	refSuffixes := []string{
		"_id", "_ids", "_arn", "_arns",
		"_name", "_names",
		"vpc_id", "subnet_id", "subnet_ids",
		"security_group_id", "security_group_ids",
		"role_arn", "target_group_arn",
		"instance_id", "cluster_arn",
		"key_id", "policy_arn",
		"source_security_group_id",
	}
	for _, suffix := range refSuffixes {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return name == "arn" || name == "akas"
}

// ---------------------------------------------------------------------------
// Query helpers
// ---------------------------------------------------------------------------

// RelatedTo returns all nodes reachable from the given key (BFS), including
// the starting node itself.
func (g *Graph) RelatedTo(startKey string) []*Node {
	start, ok := g.Nodes[startKey]
	if !ok {
		return nil
	}

	visited := map[string]bool{startKey: true}
	queue := []*Node{start}
	var result []*Node

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		for edgeKey := range current.Edges {
			if visited[edgeKey] {
				continue
			}
			visited[edgeKey] = true
			if neighbor, ok := g.Nodes[edgeKey]; ok {
				queue = append(queue, neighbor)
			}
		}
	}
	return result
}

// Roots returns nodes with no inbound edges (potential top-level resources).
func (g *Graph) Roots() []*Node {
	// Determine which nodes have inbound edges.
	hasInbound := make(map[string]bool)
	for _, node := range g.Nodes {
		for edgeKey := range node.Edges {
			hasInbound[edgeKey] = true
		}
	}

	var roots []*Node
	for key, node := range g.Nodes {
		if !hasInbound[key] {
			roots = append(roots, node)
		}
	}
	return roots
}

// ResourceTypes returns a deduplicated, sorted list of resource types in the graph.
func (g *Graph) ResourceTypes() []string {
	seen := make(map[string]bool)
	for _, node := range g.Nodes {
		seen[node.Resource.Type] = true
	}
	types := make([]string, 0, len(seen))
	for t := range seen {
		types = append(types, t)
	}
	// Sort for deterministic output.
	sortStrings(types)
	return types
}

// NodesByType returns all nodes of a given resource type.
func (g *Graph) NodesByType(resourceType string) []*Node {
	var result []*Node
	for _, node := range g.Nodes {
		if node.Resource.Type == resourceType {
			result = append(result, node)
		}
	}
	return result
}

// AllResources returns all steampipe.Resource values in the graph.
func (g *Graph) AllResources() []steampipe.Resource {
	resources := make([]steampipe.Resource, 0, len(g.Nodes))
	for _, node := range g.Nodes {
		resources = append(resources, node.Resource)
	}
	return resources
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func nodeKey(table, id string) string {
	return fmt.Sprintf("%s/%s", table, id)
}

// sortStrings sorts a slice of strings in place (simple insertion sort to avoid
// importing sort package for a small helper).
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
