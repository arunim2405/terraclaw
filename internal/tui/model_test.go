package tui

import (
	"strings"
	"testing"
)

// TestTUI_GeneratingView_Stage1 verifies that the View output contains
// "Stage 1: Generating Blueprint..." when generationStage is 1.
//
// Validates: Requirements 4.1
func TestTUI_GeneratingView_Stage1(t *testing.T) {
	t.Helper()

	m := Model{
		step:            StepGenerating,
		generationStage: 1,
		width:           80,
		height:          24,
	}

	output := m.View()

	if !strings.Contains(output, "Stage 1: Generating Blueprint...") {
		t.Errorf("expected View output to contain %q, got:\n%s",
			"Stage 1: Generating Blueprint...", output)
	}
}

// TestTUI_GeneratingView_Stage2 verifies that the View output contains
// "Stage 2: Generating Terraform Code..." when generationStage is 2.
//
// Validates: Requirements 4.2
func TestTUI_GeneratingView_Stage2(t *testing.T) {
	t.Helper()

	m := Model{
		step:            StepGenerating,
		generationStage: 2,
		width:           80,
		height:          24,
	}

	output := m.View()

	if !strings.Contains(output, "Stage 2: Generating Terraform Code...") {
		t.Errorf("expected View output to contain %q, got:\n%s",
			"Stage 2: Generating Terraform Code...", output)
	}
}
