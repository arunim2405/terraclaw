package llm

// azureRegistryModuleCatalog returns the registry module preference section
// for Azure (azurerm community modules and official HashiCorp modules).
func azureRegistryModuleCatalog() string {
	return `<registry_module_preference>
ALWAYS prefer well-maintained Azure Terraform modules from the Terraform Registry
over hand-written local modules. Use the following catalog when a matching resource
type appears in the input:

| Resource types                                         | Registry module                                          | Key inputs to map                           |
|--------------------------------------------------------|----------------------------------------------------------|---------------------------------------------|
| azure_virtual_network, azure_subnet,                   | Azure/vnet/azurerm                                       | vnet_name, address_space, subnets, tags     |
|   azure_network_security_group                         |                                                          |                                             |
| azure_storage_account (+ containers, blobs)            | Azure/storage-account/azurerm                            | name, account_tier, replication, containers |
| azure_kubernetes_cluster                               | Azure/aks/azurerm                                        | cluster_name, dns_prefix, node_pools        |
| azure_compute_virtual_machine                          | Azure/compute/azurerm                                    | name, size, image, os_disk, network         |
| azure_sql_server, azure_sql_database                   | Azure/mssql/azurerm                                      | server_name, db_name, sku, admin            |
| azure_postgresql_flexible_server                       | Azure/postgresql/azurerm                                 | server_name, sku, version, storage          |
| azure_key_vault                                        | Azure/key-vault/azurerm                                  | name, sku, access_policies, secrets         |
| azure_app_service_web_app, azure_app_service_plan      | Azure/app-service/azurerm                                | name, plan, site_config, app_settings       |
| azure_container_registry                               | Azure/container-registry/azurerm                         | name, sku, admin_enabled, geo_replications  |
| azure_cosmosdb_account                                 | Azure/cosmosdb/azurerm                                   | name, offer_type, consistency, geo_location |
| azure_lb, azure_public_ip                              | Azure/loadbalancer/azurerm                               | name, type, frontend_ip, rules, probes      |
| azure_dns_zone                                         | Azure/dns/azurerm                                        | domain_name, records                        |
| azure_log_analytics_workspace                          | Azure/log-analytics/azurerm                              | name, sku, retention_in_days                |
| azure_application_insights                             | Azure/application-insights/azurerm                       | name, application_type, workspace_id        |
| azure_servicebus_namespace                             | Azure/servicebus/azurerm                                 | name, sku, queues, topics                   |
| azure_eventhub_namespace                               | Azure/eventhub/azurerm                                   | name, sku, hubs, consumer_groups            |

If a resource type is NOT in this table, use a local module with raw resources.
When in doubt, prefer the registry module — it handles companion resources
(e.g., storage account network rules, diagnostic settings) internally.

IMPORTANT: When using a registry module, map the resource properties from the
JSON to the module's INPUT variables. Do NOT list raw resources inside a registry
module entry — the module manages them internally. Instead, provide import_mappings
that map the module's internal resource addresses to the import IDs.
</registry_module_preference>`
}

// azureModuleGroupingRules returns the module grouping instructions for Azure.
func azureModuleGroupingRules() string {
	return `<module_grouping_rules>
Group related resources into a single logical module. Registry modules already
handle this grouping internally. For local modules:
- azure_virtual_network + azure_subnet + azure_network_security_group → single VNet module
- azure_compute_virtual_machine + azure_network_interface + azure_compute_disk → single VM module
- azure_sql_server + azure_sql_database → single SQL module
- azure_app_service_web_app + azure_app_service_plan → single App Service module
- azure_kubernetes_cluster + node pools → single AKS module
- azure_storage_account + containers + blob services → single Storage module
- azure_key_vault + access policies + secrets → single Key Vault module
- azure_lb + azure_public_ip + backend pools + rules → single LB module

Module names: snake_case, descriptive, without provider prefix (e.g., vnet_main,
storage_account, aks_cluster, sql_primary). When sharing via for_each, use a
generic name (e.g., storage_account rather than storage_account_data).
</module_grouping_rules>`
}

// azureImportIDRules returns Azure-specific import ID documentation.
func azureImportIDRules() string {
	return `<import_id_rules>
Every resource MUST include import information:
- For registry modules: provide import_mappings with the module's INTERNAL
  resource address and the exact import ID from the Resource JSON.
- For local modules: provide import_id on each resource, copied EXACTLY from
  the "ID" field in the Resource JSON.

Azure resources are imported using their full Azure resource ID:
  /subscriptions/{sub-id}/resourceGroups/{rg}/providers/{provider}/{type}/{name}

Do not fabricate or modify import IDs. Use the full resource ID as provided
in the Resource JSON.
</import_id_rules>`
}

// azureCLIFallback returns the Azure CLI fallback section for Stage 1.
func azureCLIFallback() string {
	return `<az_cli_fallback>
If the Resource JSON is missing information needed for an accurate blueprint
(e.g., NSG rules, storage account keys, app settings, connection strings),
note the needed Azure CLI command in the module's description field:
  description: "AKS cluster (fetch: az aks show --name X --resource-group Y)"

Stage 2 will execute these commands before generating HCL.
</az_cli_fallback>`
}

// azureCLIDiagnostics returns the Azure CLI diagnostic commands for Stage 3.
func azureCLIDiagnostics() string {
	return `2. **Fetch actual config via Azure CLI**:
   ` + "`az vm show --name X --resource-group Y`" + `
   ` + "`az network vnet show --name X --resource-group Y`" + `
   ` + "`az network nsg show --name X --resource-group Y`" + `
   ` + "`az storage account show --name X --resource-group Y`" + `
   ` + "`az sql server show --name X --resource-group Y`" + `
   ` + "`az sql db show --server X --name Y --resource-group Z`" + `
   ` + "`az aks show --name X --resource-group Y`" + `
   ` + "`az keyvault show --name X --resource-group Y`" + `
   ` + "`az webapp show --name X --resource-group Y`" + `
   ` + "`az postgres flexible-server show --name X --resource-group Y`" + `
   ` + "`az cosmosdb show --name X --resource-group Y`" + `
   Use the appropriate CLI command for each resource type.`
}

// azureProviderBlock returns the Terraform provider configuration for Azure.
func azureProviderBlock() string {
	return `### providers.tf
- provider "azurerm" with features {} block (required)
- subscription_id from variable if needed
- default_tags block with common tags (Project = "terraclaw", ManagedBy = "terraform")`
}

// azureVersionsBlock returns the versions.tf description for Azure.
func azureVersionsBlock() string {
	return `### versions.tf
- terraform { required_version = ">= 1.5" }
- Pin AzureRM provider: source = "hashicorp/azurerm", version = "~> 4.0" (or latest stable)
- Add any other required providers (e.g., azuread, random, null) if used`
}
