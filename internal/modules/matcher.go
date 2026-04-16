package modules

import "strings"

// Scoring weights.
const (
	weightCoverage    = 0.50
	weightSpecificity = 0.30
	weightVarReady    = 0.20
)

// MatchModules scores each module against the target resource types and returns
// results sorted by score descending. Only modules with a positive score (at
// least one resource type overlap) are included.
func MatchModules(mods []ModuleMetadata, targetTypes []string, resourceProps ...map[string]map[string]string) []FitResult {
	if len(mods) == 0 || len(targetTypes) == 0 {
		return nil
	}

	targetSet := make(map[string]bool, len(targetTypes))
	for _, t := range targetTypes {
		targetSet[t] = true
	}

	// Optional property keys per resource type for variable readiness scoring.
	var propKeys map[string]map[string]string
	if len(resourceProps) > 0 {
		propKeys = resourceProps[0]
	}

	var results []FitResult
	for _, mod := range mods {
		fit := computeFitScore(mod, targetSet, propKeys)
		if fit.Score > 0 {
			// Pre-select modules with high fit.
			fit.Selected = fit.Score >= 0.60
			results = append(results, fit)
		}
	}

	// Sort by score descending (stable for equal scores by name).
	sortFitResults(results)

	return results
}

// computeFitScore calculates the three sub-scores and weighted total.
func computeFitScore(mod ModuleMetadata, targetSet map[string]bool, propKeys map[string]map[string]string) FitResult {
	moduleTypes := make(map[string]bool, len(mod.ResourceTypes))
	for _, rt := range mod.ResourceTypes {
		moduleTypes[rt] = true
	}

	// Intersection: module types that are in the target set.
	var matched []string
	for rt := range moduleTypes {
		if targetSet[rt] {
			matched = append(matched, rt)
		}
	}

	// Module types NOT in the target set.
	var unmatched []string
	for rt := range moduleTypes {
		if !targetSet[rt] {
			unmatched = append(unmatched, rt)
		}
	}

	overlap := float64(len(matched))

	// Coverage: what fraction of target types does this module cover?
	coverage := 0.0
	if len(targetSet) > 0 {
		coverage = overlap / float64(len(targetSet))
	}

	// Specificity: how focused is the module on the target types?
	specificity := 0.0
	if len(moduleTypes) > 0 {
		specificity = overlap / float64(len(moduleTypes))
	}

	// Variable readiness: can we auto-populate required variables?
	varReady := computeVarReadiness(mod, targetSet, propKeys)

	score := coverage*weightCoverage + specificity*weightSpecificity + varReady*weightVarReady

	// Determine which required inputs we can't auto-populate.
	var missingInputs []string
	if propKeys != nil {
		allProps := mergePropertyKeys(propKeys, targetSet)
		for _, v := range mod.Variables {
			if !v.Required {
				continue
			}
			if !fuzzyMatchVarToProps(v.Name, allProps) {
				missingInputs = append(missingInputs, v.Name)
			}
		}
	} else {
		missingInputs = mod.RequiredInputs()
	}

	return FitResult{
		Module:           mod,
		Score:            score,
		CoverageScore:    coverage,
		SpecificityScore: specificity,
		VarReadiness:     varReady,
		MatchedTypes:     matched,
		UnmatchedModule:  unmatched,
		MissingInputs:    missingInputs,
	}
}

// computeVarReadiness scores how many of the module's variables are either
// optional (have defaults) or can be matched to resource property keys.
func computeVarReadiness(mod ModuleMetadata, targetSet map[string]bool, propKeys map[string]map[string]string) float64 {
	if len(mod.Variables) == 0 {
		return 1.0 // No variables needed — trivially ready.
	}

	required := mod.RequiredInputs()
	if len(required) == 0 {
		return 1.0 // All variables have defaults.
	}

	if propKeys == nil {
		// No resource properties available — score based on ratio of optional vars.
		optional := float64(len(mod.Variables) - len(required))
		return optional / float64(len(mod.Variables))
	}

	// Collect all property keys from target resources.
	allProps := mergePropertyKeys(propKeys, targetSet)

	matchedCount := 0
	for _, varName := range required {
		if fuzzyMatchVarToProps(varName, allProps) {
			matchedCount++
		}
	}

	return float64(matchedCount) / float64(len(required))
}

// mergePropertyKeys collects all property keys from target resource types into a set.
func mergePropertyKeys(propKeys map[string]map[string]string, targetSet map[string]bool) map[string]bool {
	result := make(map[string]bool)
	for rt, props := range propKeys {
		if !targetSet[rt] {
			continue
		}
		for key := range props {
			result[normalizeKey(key)] = true
		}
	}
	return result
}

// fuzzyMatchVarToProps checks if a variable name fuzzy-matches any property key.
func fuzzyMatchVarToProps(varName string, propKeys map[string]bool) bool {
	normalized := normalizeKey(varName)
	if propKeys[normalized] {
		return true
	}
	for pk := range propKeys {
		if strings.Contains(pk, normalized) || strings.Contains(normalized, pk) {
			return true
		}
	}
	return false
}

// normalizeKey normalizes a key for fuzzy comparison: lowercase, strip
// underscores and hyphens.
func normalizeKey(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, "-", "")
	return s
}

// sortFitResults sorts by score descending, then by name ascending for ties.
func sortFitResults(results []FitResult) {
	for i := 1; i < len(results); i++ {
		for j := i; j > 0; j-- {
			if results[j].Score > results[j-1].Score {
				results[j], results[j-1] = results[j-1], results[j]
			} else if results[j].Score == results[j-1].Score && results[j].Module.Name < results[j-1].Module.Name {
				results[j], results[j-1] = results[j-1], results[j]
			} else {
				break
			}
		}
	}
}
