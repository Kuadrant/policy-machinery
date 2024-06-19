//go:build unit

package color

import (
	"testing"

	machinery "github.com/guicassolato/policy-machinery"
)

func TestMerge(t *testing.T) {
	testCases := []struct {
		name     string
		source   machinery.Policy
		target   machinery.Policy
		strategy machinery.MergeStrategy
		expected map[machinery.RuleId]ColorValue
	}{
		{
			name: "Merge atomic defaults into empty policy",
			source: &ColorPolicy{
				Spec: ColorSpec{
					Rules: []ColorRule{
						{
							Id:    "rule-1",
							Color: Blue,
						},
						{
							Id:    "rule-2",
							Color: Red,
						},
					},
				},
			},
			target:   &ColorPolicy{},
			strategy: machinery.AtomicDefaultsMergeStrategy,
			expected: map[machinery.RuleId]ColorValue{
				"rule-1": Blue,
				"rule-2": Red,
			},
		},
		{
			name: "Merge atomic defaults into policy without conflicting rules",
			source: &ColorPolicy{
				Spec: ColorSpec{
					Rules: []ColorRule{
						{
							Id:    "rule-1",
							Color: Blue,
						},
						{
							Id:    "rule-2",
							Color: Red,
						},
					},
				},
			},
			target: &ColorPolicy{
				Spec: ColorSpec{
					Rules: []ColorRule{
						{
							Id:    "rule-3",
							Color: Green,
						},
					},
				},
			},
			strategy: machinery.AtomicDefaultsMergeStrategy,
			expected: map[machinery.RuleId]ColorValue{
				"rule-3": Green,
			},
		},
		{
			name: "Merge atomic defaults into policy with conflicting rules",
			source: &ColorPolicy{
				Spec: ColorSpec{
					Rules: []ColorRule{
						{
							Id:    "rule-1",
							Color: Blue,
						},
						{
							Id:    "rule-2",
							Color: Red,
						},
					},
				},
			},
			target: &ColorPolicy{
				Spec: ColorSpec{
					Rules: []ColorRule{
						{
							Id:    "rule-1",
							Color: Yellow,
						},
						{
							Id:    "rule-3",
							Color: Green,
						},
					},
				},
			},
			strategy: machinery.AtomicDefaultsMergeStrategy,
			expected: map[machinery.RuleId]ColorValue{
				"rule-1": Yellow,
				"rule-3": Green,
			},
		},
		{
			name: "Merge atomic overrides into empty policy",
			source: &ColorPolicy{
				Spec: ColorSpec{
					Rules: []ColorRule{
						{
							Id:    "rule-1",
							Color: Blue,
						},
						{
							Id:    "rule-2",
							Color: Red,
						},
					},
				},
			},
			target:   &ColorPolicy{},
			strategy: machinery.AtomicOverridesMergeStrategy,
			expected: map[machinery.RuleId]ColorValue{
				"rule-1": Blue,
				"rule-2": Red,
			},
		},
		{
			name: "Merge atomic overrides into policy without conflicting rules",
			source: &ColorPolicy{
				Spec: ColorSpec{
					Rules: []ColorRule{
						{
							Id:    "rule-1",
							Color: Blue,
						},
						{
							Id:    "rule-2",
							Color: Red,
						},
					},
				},
			},
			target: &ColorPolicy{
				Spec: ColorSpec{
					Rules: []ColorRule{
						{
							Id:    "rule-3",
							Color: Green,
						},
					},
				},
			},
			strategy: machinery.AtomicOverridesMergeStrategy,
			expected: map[machinery.RuleId]ColorValue{
				"rule-1": Blue,
				"rule-2": Red,
			},
		},
		{
			name: "Merge atomic overrides into policy with conflicting rules",
			source: &ColorPolicy{
				Spec: ColorSpec{
					Rules: []ColorRule{
						{
							Id:    "rule-1",
							Color: Blue,
						},
						{
							Id:    "rule-2",
							Color: Red,
						},
					},
				},
			},
			target: &ColorPolicy{
				Spec: ColorSpec{
					Rules: []ColorRule{
						{
							Id:    "rule-1",
							Color: Yellow,
						},
						{
							Id:    "rule-3",
							Color: Green,
						},
					},
				},
			},
			strategy: machinery.AtomicOverridesMergeStrategy,
			expected: map[machinery.RuleId]ColorValue{
				"rule-1": Blue,
				"rule-2": Red,
			},
		},
		{
			name: "Merge policy rule defaults into empty policy",
			source: &ColorPolicy{
				Spec: ColorSpec{
					Rules: []ColorRule{
						{
							Id:    "rule-1",
							Color: Blue,
						},
						{
							Id:    "rule-2",
							Color: Red,
						},
					},
				},
			},
			target:   &ColorPolicy{},
			strategy: machinery.PolicyRuleDefaultsMergeStrategy,
			expected: map[machinery.RuleId]ColorValue{
				"rule-1": Blue,
				"rule-2": Red,
			},
		},
		{
			name: "Merge policy rule defaults into policy without conflicting rules",
			source: &ColorPolicy{
				Spec: ColorSpec{
					Rules: []ColorRule{
						{
							Id:    "rule-1",
							Color: Blue,
						},
						{
							Id:    "rule-2",
							Color: Red,
						},
					},
				},
			},
			target: &ColorPolicy{
				Spec: ColorSpec{
					Rules: []ColorRule{
						{
							Id:    "rule-3",
							Color: Green,
						},
					},
				},
			},
			strategy: machinery.PolicyRuleDefaultsMergeStrategy,
			expected: map[machinery.RuleId]ColorValue{
				"rule-1": Blue,
				"rule-2": Red,
				"rule-3": Green,
			},
		},
		{
			name: "Merge policy rule defaults into policy with conflicting rules",
			source: &ColorPolicy{
				Spec: ColorSpec{
					Rules: []ColorRule{
						{
							Id:    "rule-1",
							Color: Blue,
						},
						{
							Id:    "rule-2",
							Color: Red,
						},
					},
				},
			},
			target: &ColorPolicy{
				Spec: ColorSpec{
					Rules: []ColorRule{
						{
							Id:    "rule-1",
							Color: Yellow,
						},
						{
							Id:    "rule-3",
							Color: Green,
						},
					},
				},
			},
			strategy: machinery.PolicyRuleDefaultsMergeStrategy,
			expected: map[machinery.RuleId]ColorValue{
				"rule-1": Yellow,
				"rule-2": Red,
				"rule-3": Green,
			},
		},
		{
			name: "Merge policy rule overrides into empty policy",
			source: &ColorPolicy{
				Spec: ColorSpec{
					Rules: []ColorRule{
						{
							Id:    "rule-1",
							Color: Blue,
						},
						{
							Id:    "rule-2",
							Color: Red,
						},
					},
				},
			},
			target:   &ColorPolicy{},
			strategy: machinery.PolicyRuleOverridesMergeStrategy,
			expected: map[machinery.RuleId]ColorValue{
				"rule-1": Blue,
				"rule-2": Red,
			},
		},
		{
			name: "Merge policy rule overrides into policy without conflicting rules",
			source: &ColorPolicy{
				Spec: ColorSpec{
					Rules: []ColorRule{
						{
							Id:    "rule-1",
							Color: Blue,
						},
						{
							Id:    "rule-2",
							Color: Red,
						},
					},
				},
			},
			target: &ColorPolicy{
				Spec: ColorSpec{
					Rules: []ColorRule{
						{
							Id:    "rule-3",
							Color: Green,
						},
					},
				},
			},
			strategy: machinery.PolicyRuleOverridesMergeStrategy,
			expected: map[machinery.RuleId]ColorValue{
				"rule-1": Blue,
				"rule-2": Red,
				"rule-3": Green,
			},
		},
		{
			name: "Merge policy rule overrides into policy with conflicting rules",
			source: &ColorPolicy{
				Spec: ColorSpec{
					Rules: []ColorRule{
						{
							Id:    "rule-1",
							Color: Blue,
						},
						{
							Id:    "rule-2",
							Color: Red,
						},
					},
				},
			},
			target: &ColorPolicy{
				Spec: ColorSpec{
					Rules: []ColorRule{
						{
							Id:    "rule-1",
							Color: Yellow,
						},
						{
							Id:    "rule-3",
							Color: Green,
						},
					},
				},
			},
			strategy: machinery.PolicyRuleOverridesMergeStrategy,
			expected: map[machinery.RuleId]ColorValue{
				"rule-1": Blue,
				"rule-2": Red,
				"rule-3": Green,
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			merged := tc.target.Merge(tc.source, tc.strategy)
			if len(merged.GetSpec().GetRules()) != len(tc.expected) {
				t.Errorf("Expected %d rules, but got %d", len(tc.expected), len(merged.GetSpec().GetRules()))
			}
			for _, rule := range merged.GetSpec().GetRules() {
				colorRule := rule.(*ColorRule)
				if tc.expected[rule.GetId()] != colorRule.Color {
					t.Errorf("Expected rule %s to have color %d, but got %d", rule.GetId(), tc.expected[rule.GetId()], colorRule.Color)
				}
			}
		})
	}
}
