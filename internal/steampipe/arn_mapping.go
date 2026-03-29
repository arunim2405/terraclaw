package steampipe

import "strings"

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
