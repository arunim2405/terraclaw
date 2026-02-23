package terraform_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/arunim2405/terraclaw/internal/steampipe"
	tf "github.com/arunim2405/terraclaw/internal/terraform"
)

func TestGuessResourceAddress(t *testing.T) {
	tests := []struct {
		name     string
		resource steampipe.Resource
		want     string
	}{
		{
			name: "aws s3 bucket with name",
			resource: steampipe.Resource{
				Provider: "aws",
				Type:     "aws_s3_bucket",
				Name:     "my-bucket",
				ID:       "my-bucket",
			},
			want: "aws_s3_bucket.my_bucket",
		},
		{
			name: "aws ec2 instance",
			resource: steampipe.Resource{
				Provider: "aws",
				Type:     "aws_ec2_instance",
				Name:     "web-server",
				ID:       "i-1234567890abcdef0",
			},
			want: "aws_ec2_instance.web_server",
		},
		{
			name: "resource without provider prefix",
			resource: steampipe.Resource{
				Provider: "aws",
				Type:     "s3_bucket",
				Name:     "my-bucket",
				ID:       "my-bucket",
			},
			want: "aws_s3_bucket.my_bucket",
		},
		{
			name: "resource with numeric name falls back to id label",
			resource: steampipe.Resource{
				Provider: "aws",
				Type:     "aws_vpc",
				Name:     "123",
				ID:       "vpc-abc",
			},
			want: "aws_vpc._123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tf.GuessResourceAddress(tt.resource)
			if got != tt.want {
				t.Errorf("GuessResourceAddress() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestImportCommand(t *testing.T) {
	r := steampipe.Resource{
		Provider: "aws",
		Type:     "aws_s3_bucket",
		Name:     "my-bucket",
		ID:       "my-bucket",
	}
	got := tf.ImportCommand("terraform", r, "aws_s3_bucket.my_bucket")
	want := "terraform import aws_s3_bucket.my_bucket my-bucket"
	if got != want {
		t.Errorf("ImportCommand() = %q, want %q", got, want)
	}
}

func TestSummaryText(t *testing.T) {
	results := []tf.ImportResult{
		{Address: "aws_s3_bucket.my_bucket", Error: nil},
		{Address: "aws_vpc.main", Error: fmt.Errorf("not found")},
	}
	summary := tf.SummaryText(results)
	if !strings.Contains(summary, "✓ aws_s3_bucket.my_bucket") {
		t.Errorf("SummaryText missing success entry, got: %s", summary)
	}
	if !strings.Contains(summary, "✗ aws_vpc.main") {
		t.Errorf("SummaryText missing failure entry, got: %s", summary)
	}
	if !strings.Contains(summary, "1 imported, 1 failed") {
		t.Errorf("SummaryText missing summary line, got: %s", summary)
	}
}
