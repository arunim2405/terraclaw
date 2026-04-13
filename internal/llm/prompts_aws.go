package llm

// awsRegistryModuleCatalog returns the registry module preference section for AWS.
func awsRegistryModuleCatalog() string {
	return `<registry_module_preference>
ALWAYS prefer official terraform-aws-modules from the Terraform Registry over
hand-written local modules. Use the following catalog when a matching resource
type appears in the input:

| Resource types                                  | Registry module                                  | Key inputs to map                         |
|-------------------------------------------------|--------------------------------------------------|-------------------------------------------|
| aws_vpc, aws_subnet, aws_internet_gateway,      | terraform-aws-modules/vpc/aws                    | cidr, azs, public/private subnets, tags   |
|   aws_nat_gateway, aws_route_table              |                                                  |                                           |
| aws_s3_bucket (+ companion resources)           | terraform-aws-modules/s3-bucket/aws              | bucket, versioning, encryption, lifecycle |
| aws_iam_role, aws_iam_policy,                   | terraform-aws-modules/iam/aws//modules/          | role name, policy JSON, trusted entities  |
|   aws_iam_role_policy_attachment                 |   iam-assumable-role                             |                                           |
| aws_lambda_function, aws_lambda_permission,     | terraform-aws-modules/lambda/aws                 | function_name, handler, runtime, env vars |
|   aws_cloudwatch_log_group                       |                                                  |                                           |
| aws_security_group, aws_security_group_rule     | terraform-aws-modules/security-group/aws         | vpc_id, ingress/egress rules              |
| aws_db_instance, aws_db_subnet_group,           | terraform-aws-modules/rds/aws                    | engine, instance_class, storage, vpc      |
|   aws_db_parameter_group                         |                                                  |                                           |
| aws_instance                                     | terraform-aws-modules/ec2-instance/aws           | ami, instance_type, subnet, sg, key       |
| aws_lb, aws_lb_listener, aws_lb_target_group    | terraform-aws-modules/alb/aws                    | vpc_id, subnets, listeners, targets       |
| aws_eks_cluster, aws_eks_node_group             | terraform-aws-modules/eks/aws                    | cluster_name, vpc_id, subnets, node groups|
| aws_dynamodb_table                               | terraform-aws-modules/dynamodb-table/aws         | name, hash_key, attributes, billing       |
| aws_sns_topic, aws_sns_topic_subscription       | terraform-aws-modules/sns/aws                    | name, subscriptions                       |
| aws_sqs_queue                                    | terraform-aws-modules/sqs/aws                    | name, visibility_timeout, dlq             |
| aws_acm_certificate                              | terraform-aws-modules/acm/aws                    | domain_name, SANs, validation_method      |
| aws_kms_key, aws_kms_alias                      | terraform-aws-modules/kms/aws                    | description, key_policy, aliases          |
| aws_autoscaling_group, aws_launch_template      | terraform-aws-modules/autoscaling/aws            | min/max/desired, launch_template, vpc     |
| aws_ecs_cluster, aws_ecs_service,               | terraform-aws-modules/ecs/aws                    | cluster_name, services, task definitions  |
|   aws_ecs_task_definition                        |                                                  |                                           |
| aws_cloudfront_distribution                      | terraform-aws-modules/cloudfront/aws             | origins, behaviors, aliases, certs        |
| aws_apigatewayv2_api                             | terraform-aws-modules/apigateway-v2/aws          | name, protocol, routes, integrations      |
| aws_sfn_state_machine                            | terraform-aws-modules/step-functions/aws         | name, definition, role_arn                |
| aws_cloudwatch_event_rule                        | terraform-aws-modules/eventbridge/aws            | rules, targets                            |

If a resource type is NOT in this table, use a local module with raw resources.
When in doubt, prefer the registry module — it handles companion resources
(e.g., S3 bucket versioning, public access block, encryption) internally.

IMPORTANT: When using a registry module, map the resource properties from the
JSON to the module's INPUT variables. Do NOT list raw resources inside a registry
module entry — the module manages them internally. Instead, provide import_mappings
that map the module's internal resource addresses to the import IDs.
</registry_module_preference>`
}

// awsModuleGroupingRules returns the module grouping instructions for AWS.
func awsModuleGroupingRules() string {
	return `<module_grouping_rules>
Group related resources into a single logical module. Registry modules already
handle this grouping internally. For local modules:
- aws_iam_role + aws_iam_role_policy + aws_iam_role_policy_attachment → single IAM module
- aws_lambda_function + aws_lambda_permission + aws_cloudwatch_log_group → single Lambda module
- aws_s3_bucket + companion resources (versioning, encryption, ACL, lifecycle, public access block) → single S3 module
- aws_security_group + aws_security_group_rule → single SG module
- aws_db_instance + aws_db_subnet_group + aws_db_parameter_group → single RDS module
- aws_vpc + aws_subnet + aws_internet_gateway + aws_nat_gateway + aws_route_table → single VPC module
- aws_ecs_cluster + aws_ecs_service + aws_ecs_task_definition → single ECS module
- aws_lb + aws_lb_listener + aws_lb_target_group → single ALB module

Module names: snake_case, descriptive, without provider prefix (e.g., vpc_main,
s3_bucket, iam_lambda_exec, rds_primary). When sharing via for_each, use a
generic name (e.g., s3_bucket rather than s3_bucket_data).
</module_grouping_rules>`
}

// awsImportIDRules returns the AWS-specific import ID documentation.
func awsImportIDRules() string {
	return `<import_id_rules>
Every resource MUST include import information:
- For registry modules: provide import_mappings with the module's INTERNAL
  resource address (e.g., "aws_vpc.this[0]", "aws_s3_bucket.this[0]") and the
  exact import ID from the Resource JSON.
- For local modules: provide import_id on each resource, copied EXACTLY from
  the "ID" field in the Resource JSON.

Common internal addresses for terraform-aws-modules:
  - vpc/aws:            aws_vpc.this[0], aws_subnet.public[*], aws_subnet.private[*],
                        aws_internet_gateway.this[0], aws_nat_gateway.this[*]
  - s3-bucket/aws:      aws_s3_bucket.this[0], aws_s3_bucket_versioning.this[0],
                        aws_s3_bucket_server_side_encryption_configuration.this[0],
                        aws_s3_bucket_public_access_block.this[0]
  - lambda/aws:         aws_lambda_function.this[0], aws_cloudwatch_log_group.lambda[0]
  - security-group/aws: aws_security_group.this_name_prefix[0] or aws_security_group.this[0]
  - rds/aws:            aws_db_instance.this[0], aws_db_subnet_group.this[0],
                        aws_db_parameter_group.this[0]
  - iam/.../iam-assumable-role: aws_iam_role.this[0], aws_iam_policy.policy[0]
  - ec2-instance/aws:   aws_instance.this[0]

Do not fabricate or modify import IDs. Use ARN if the JSON provides ARN, name
if it provides name.
</import_id_rules>`
}

// awsCLIFallback returns the AWS CLI fallback section for Stage 1.
func awsCLIFallback() string {
	return `<aws_cli_fallback>
If the Resource JSON is missing information needed for an accurate blueprint
(e.g., IAM policy documents, security group rules, subnet AZs, Lambda
environment variables, RDS parameter groups), note the needed AWS CLI command
in the module's description field:
  description: "Lambda function (fetch: aws lambda get-function --function-name X)"

Stage 2 will execute these commands before generating HCL.
</aws_cli_fallback>`
}

// awsCLIDiagnostics returns the AWS CLI diagnostic commands for Stage 3.
func awsCLIDiagnostics() string {
	return `2. **Fetch actual config via AWS CLI**:
   ` + "`aws s3api get-bucket-versioning --bucket X`" + `
   ` + "`aws s3api get-bucket-encryption --bucket X`" + `
   ` + "`aws s3api get-public-access-block --bucket X`" + `
   ` + "`aws iam get-role --role-name X`" + `
   ` + "`aws iam list-attached-role-policies --role-name X`" + `
   ` + "`aws lambda get-function --function-name X`" + `
   ` + "`aws ec2 describe-vpcs --vpc-ids X`" + `
   ` + "`aws ec2 describe-subnets --filters Name=vpc-id,Values=X`" + `
   ` + "`aws ec2 describe-security-groups --group-ids X`" + `
   ` + "`aws rds describe-db-instances --db-instance-identifier X`" + `
   ` + "`aws cognito-idp describe-user-pool --user-pool-id X`" + `
   Use the appropriate CLI command for each resource type.`
}

// awsProviderBlock returns the Terraform provider configuration for AWS.
func awsProviderBlock() string {
	return `### providers.tf
- provider "aws" {} with region + default_tags
- default_tags block with common tags (Project = "terraclaw", ManagedBy = "terraform")`
}

// awsVersionsBlock returns the versions.tf description for AWS.
func awsVersionsBlock() string {
	return `### versions.tf
- terraform { required_version = ">= 1.5" }
- Pin AWS provider: source = "hashicorp/aws", version = "~> 5.0" (or latest stable)
- Add any other required providers (e.g., random, null, tls) if used`
}
