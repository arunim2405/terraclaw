package steampipe

import "strings"

// ---------------------------------------------------------------------------
// AWS ARN → Steampipe table mapping
// ---------------------------------------------------------------------------

// arnServiceToTable maps the service segment of an AWS ARN to the
// corresponding Steampipe table name. This allows terraclaw to look up
// resources by ARN without scanning all tables.
//
// An ARN has the format: arn:partition:service:region:account:resource
// We key off the service segment (index 2 after splitting on ':').
var arnServiceToTable = map[string][]string{
	"cognito-idp":       {"aws_cognito_user_pool"},
	"cognito-identity":  {"aws_cognito_identity_pool"},
	"ec2":               {"aws_ec2_instance", "aws_vpc", "aws_subnet", "aws_security_group", "aws_internet_gateway", "aws_nat_gateway", "aws_route_table", "aws_network_interface", "aws_eip", "aws_ec2_launch_template", "aws_ebs_volume", "aws_ec2_application_load_balancer", "aws_ec2_network_load_balancer", "aws_ec2_target_group"},
	"lambda":            {"aws_lambda_function"},
	"ecs":               {"aws_ecs_cluster", "aws_ecs_service", "aws_ecs_task_definition"},
	"iam":               {"aws_iam_role", "aws_iam_policy", "aws_iam_user", "aws_iam_group", "aws_iam_instance_profile"},
	"s3":                {"aws_s3_bucket"},
	"elasticfilesystem": {"aws_efs_file_system"},
	"rds":               {"aws_rds_db_instance", "aws_rds_db_cluster"},
	"dynamodb":          {"aws_dynamodb_table"},
	"elasticache":       {"aws_elasticache_cluster"},
	"route53":           {"aws_route53_zone"},
	"kms":               {"aws_kms_key"},
	"sns":               {"aws_sns_topic"},
	"sqs":               {"aws_sqs_queue"},
	"logs":              {"aws_cloudwatch_log_group"},
	"autoscaling":       {"aws_autoscaling_group"},
}

// TableNamesForARN returns the Steampipe table names to search for a given ARN.
// If the ARN service is not recognised, it returns nil.
func TableNamesForARN(arn string) []string {
	parts := strings.SplitN(arn, ":", 6)
	if len(parts) < 3 {
		return nil
	}
	service := parts[2]
	return arnServiceToTable[service]
}

// ---------------------------------------------------------------------------
// Azure resource ID → Steampipe table mapping
// ---------------------------------------------------------------------------

// azureProviderToTable maps the Azure resource provider/type segment of a
// resource ID to Steampipe table names.
//
// An Azure resource ID has the format:
//
//	/subscriptions/{sub}/resourceGroups/{rg}/providers/{provider}/{type}/{name}
//
// We key off the "providers/{provider}/{type}" segment (lowercased).
var azureProviderToTable = map[string][]string{
	// Compute
	"microsoft.compute/virtualmachines":                 {"azure_compute_virtual_machine"},
	"microsoft.compute/virtualmachinescalesets":         {"azure_compute_virtual_machine_scale_set"},
	"microsoft.compute/disks":                          {"azure_compute_disk"},
	"microsoft.compute/availabilitysets":                {"azure_compute_availability_set"},
	"microsoft.compute/images":                         {"azure_compute_image"},
	"microsoft.compute/snapshots":                      {"azure_compute_snapshot"},

	// Networking
	"microsoft.network/virtualnetworks":                {"azure_virtual_network"},
	"microsoft.network/networksecuritygroups":          {"azure_network_security_group"},
	"microsoft.network/publicipaddresses":              {"azure_public_ip"},
	"microsoft.network/loadbalancers":                  {"azure_lb"},
	"microsoft.network/applicationgateways":            {"azure_application_gateway"},
	"microsoft.network/networkinterfaces":              {"azure_network_interface"},
	"microsoft.network/routetables":                    {"azure_route_table"},
	"microsoft.network/natgateways":                    {"azure_nat_gateway"},
	"microsoft.network/dnszones":                       {"azure_dns_zone"},
	"microsoft.network/privatednszones":                {"azure_private_dns_zone"},
	"microsoft.network/frontdoors":                     {"azure_frontdoor"},
	"microsoft.network/virtualnetworkgateways":         {"azure_virtual_network_gateway"},

	// Storage
	"microsoft.storage/storageaccounts":                {"azure_storage_account"},

	// Databases
	"microsoft.sql/servers":                            {"azure_sql_server"},
	"microsoft.sql/servers/databases":                  {"azure_sql_database"},
	"microsoft.dbforpostgresql/flexibleservers":        {"azure_postgresql_flexible_server"},
	"microsoft.dbformysql/flexibleservers":             {"azure_mysql_flexible_server"},
	"microsoft.documentdb/databaseaccounts":            {"azure_cosmosdb_account"},
	"microsoft.cache/redis":                            {"azure_redis_cache"},

	// Containers
	"microsoft.containerservice/managedclusters":       {"azure_kubernetes_cluster"},
	"microsoft.containerregistry/registries":           {"azure_container_registry"},
	"microsoft.containerinstance/containergroups":      {"azure_container_group"},

	// App Services
	"microsoft.web/sites":                              {"azure_app_service_web_app"},
	"microsoft.web/serverfarms":                        {"azure_app_service_plan"},

	// Identity & Access
	"microsoft.managedidentity/userassignedidentities": {"azure_managed_identity"},

	// Key Vault
	"microsoft.keyvault/vaults":                        {"azure_key_vault"},

	// Monitoring
	"microsoft.insights/components":                    {"azure_application_insights"},
	"microsoft.operationalinsights/workspaces":         {"azure_log_analytics_workspace"},

	// Messaging
	"microsoft.servicebus/namespaces":                  {"azure_servicebus_namespace"},
	"microsoft.eventhub/namespaces":                    {"azure_eventhub_namespace"},

	// Resource Management
	"microsoft.resources/resourcegroups":               {"azure_resource_group"},
}

// TableNamesForAzureResourceID returns the Steampipe table names for a given
// Azure resource ID. It extracts the provider/type segment and looks it up in
// the mapping. Returns nil if the resource type is not recognised.
func TableNamesForAzureResourceID(resourceID string) []string {
	providerType := extractAzureProviderType(resourceID)
	if providerType == "" {
		return nil
	}
	return azureProviderToTable[providerType]
}

// extractAzureProviderType extracts the "microsoft.xxx/yyy" provider/type from
// an Azure resource ID. Returns empty string if the format is not recognised.
//
// Example input:  /subscriptions/xxx/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/vm1
// Example output: microsoft.compute/virtualmachines
func extractAzureProviderType(resourceID string) string {
	lower := strings.ToLower(resourceID)
	idx := strings.Index(lower, "/providers/")
	if idx == -1 {
		return ""
	}
	rest := lower[idx+len("/providers/"):]

	parts := strings.Split(rest, "/")
	if len(parts) < 2 {
		return ""
	}

	// Try sub-resource pattern first: provider/type/name/subtype
	// e.g. microsoft.sql/servers/myserver/databases → microsoft.sql/servers/databases
	if len(parts) >= 4 {
		subType := parts[0] + "/" + parts[1] + "/" + parts[3]
		if _, ok := azureProviderToTable[subType]; ok {
			return subType
		}
	}

	return parts[0] + "/" + parts[1]
}

// ---------------------------------------------------------------------------
// Unified resource ID → table lookup
// ---------------------------------------------------------------------------

// TableNamesForResourceID returns the Steampipe table names for a given
// resource identifier, auto-detecting whether it is an AWS ARN or Azure
// resource ID based on its format prefix.
func TableNamesForResourceID(id string) []string {
	switch {
	case strings.HasPrefix(id, "arn:"):
		return TableNamesForARN(id)
	case strings.HasPrefix(strings.ToLower(id), "/subscriptions/"):
		return TableNamesForAzureResourceID(id)
	default:
		return nil
	}
}
