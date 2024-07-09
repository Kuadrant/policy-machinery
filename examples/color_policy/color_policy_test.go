//go:build unit

package color_policy

import (
	"testing"

	"github.com/kuadrant/policy-machinery/machinery"
)

func TestMerge(t *testing.T) {
	testCases := []struct {
		name     string
		source   machinery.Policy
		target   machinery.Policy
		expected map[string]ColorValue
	}{
		{
			name: "Merge atomic defaults into empty policy",
			source: &ColorPolicy{
				Spec: ColorSpec{
					Defaults: &MergeableColorSpec{
						Strategy: AtomicMergeStrategy,
						ColorSpecProper: ColorSpecProper{
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
				},
			},
			target: &ColorPolicy{},
			expected: map[string]ColorValue{
				"rule-1": Blue,
				"rule-2": Red,
			},
		},
		{
			name: "Merge atomic defaults into policy without conflicting rules",
			source: &ColorPolicy{
				Spec: ColorSpec{
					Defaults: &MergeableColorSpec{
						Strategy: AtomicMergeStrategy,
						ColorSpecProper: ColorSpecProper{
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
				},
			},
			target: &ColorPolicy{
				Spec: ColorSpec{
					ColorSpecProper: ColorSpecProper{
						Rules: []ColorRule{
							{
								Id:    "rule-3",
								Color: Green,
							},
						},
					},
				},
			},
			expected: map[string]ColorValue{
				"rule-3": Green,
			},
		},
		{
			name: "Merge atomic defaults into policy with conflicting rules",
			source: &ColorPolicy{
				Spec: ColorSpec{
					Defaults: &MergeableColorSpec{
						Strategy: AtomicMergeStrategy,
						ColorSpecProper: ColorSpecProper{
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
				},
			},
			target: &ColorPolicy{
				Spec: ColorSpec{
					ColorSpecProper: ColorSpecProper{
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
			},
			expected: map[string]ColorValue{
				"rule-1": Yellow,
				"rule-3": Green,
			},
		},
		{
			name: "Merge atomic overrides into empty policy",
			source: &ColorPolicy{
				Spec: ColorSpec{
					Overrides: &MergeableColorSpec{
						Strategy: AtomicMergeStrategy,
						ColorSpecProper: ColorSpecProper{
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
				},
			},
			target: &ColorPolicy{},
			expected: map[string]ColorValue{
				"rule-1": Blue,
				"rule-2": Red,
			},
		},
		{
			name: "Merge atomic overrides into policy without conflicting rules",
			source: &ColorPolicy{
				Spec: ColorSpec{
					Overrides: &MergeableColorSpec{
						Strategy: AtomicMergeStrategy,
						ColorSpecProper: ColorSpecProper{
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
				},
			},
			target: &ColorPolicy{
				Spec: ColorSpec{
					ColorSpecProper: ColorSpecProper{
						Rules: []ColorRule{
							{
								Id:    "rule-3",
								Color: Green,
							},
						},
					},
				},
			},
			expected: map[string]ColorValue{
				"rule-1": Blue,
				"rule-2": Red,
			},
		},
		{
			name: "Merge atomic overrides into policy with conflicting rules",
			source: &ColorPolicy{
				Spec: ColorSpec{
					Overrides: &MergeableColorSpec{
						Strategy: AtomicMergeStrategy,
						ColorSpecProper: ColorSpecProper{
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
				},
			},
			target: &ColorPolicy{
				Spec: ColorSpec{
					ColorSpecProper: ColorSpecProper{
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
			},
			expected: map[string]ColorValue{
				"rule-1": Blue,
				"rule-2": Red,
			},
		},
		{
			name: "Merge policy rule defaults into empty policy",
			source: &ColorPolicy{
				Spec: ColorSpec{
					Defaults: &MergeableColorSpec{
						Strategy: PolicyRuleMergeStrategy,
						ColorSpecProper: ColorSpecProper{
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
				},
			},
			target: &ColorPolicy{},
			expected: map[string]ColorValue{
				"rule-1": Blue,
				"rule-2": Red,
			},
		},
		{
			name: "Merge policy rule defaults into policy without conflicting rules",
			source: &ColorPolicy{
				Spec: ColorSpec{
					Defaults: &MergeableColorSpec{
						Strategy: PolicyRuleMergeStrategy,
						ColorSpecProper: ColorSpecProper{
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
				},
			},
			target: &ColorPolicy{
				Spec: ColorSpec{
					ColorSpecProper: ColorSpecProper{
						Rules: []ColorRule{
							{
								Id:    "rule-3",
								Color: Green,
							},
						},
					},
				},
			},
			expected: map[string]ColorValue{
				"rule-1": Blue,
				"rule-2": Red,
				"rule-3": Green,
			},
		},
		{
			name: "Merge policy rule defaults into policy with conflicting rules",
			source: &ColorPolicy{
				Spec: ColorSpec{
					Defaults: &MergeableColorSpec{
						Strategy: PolicyRuleMergeStrategy,
						ColorSpecProper: ColorSpecProper{
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
				},
			},
			target: &ColorPolicy{
				Spec: ColorSpec{
					ColorSpecProper: ColorSpecProper{
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
			},
			expected: map[string]ColorValue{
				"rule-1": Yellow,
				"rule-2": Red,
				"rule-3": Green,
			},
		},
		{
			name: "Merge policy rule overrides into empty policy",
			source: &ColorPolicy{
				Spec: ColorSpec{
					Overrides: &MergeableColorSpec{
						Strategy: PolicyRuleMergeStrategy,
						ColorSpecProper: ColorSpecProper{
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
				},
			},
			target: &ColorPolicy{},
			expected: map[string]ColorValue{
				"rule-1": Blue,
				"rule-2": Red,
			},
		},
		{
			name: "Merge policy rule overrides into policy without conflicting rules",
			source: &ColorPolicy{
				Spec: ColorSpec{
					Overrides: &MergeableColorSpec{
						Strategy: PolicyRuleMergeStrategy,
						ColorSpecProper: ColorSpecProper{
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
				},
			},
			target: &ColorPolicy{
				Spec: ColorSpec{
					ColorSpecProper: ColorSpecProper{
						Rules: []ColorRule{
							{
								Id:    "rule-3",
								Color: Green,
							},
						},
					},
				},
			},
			expected: map[string]ColorValue{
				"rule-1": Blue,
				"rule-2": Red,
				"rule-3": Green,
			},
		},
		{
			name: "Merge policy rule overrides into policy with conflicting rules",
			source: &ColorPolicy{
				Spec: ColorSpec{
					Overrides: &MergeableColorSpec{
						Strategy: PolicyRuleMergeStrategy,
						ColorSpecProper: ColorSpecProper{
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
				},
			},
			target: &ColorPolicy{
				Spec: ColorSpec{
					ColorSpecProper: ColorSpecProper{
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
			},
			expected: map[string]ColorValue{
				"rule-1": Blue,
				"rule-2": Red,
				"rule-3": Green,
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			merged := tc.target.Merge(tc.source)
			mergedColorPolicy := merged.(*ColorPolicy)
			mergedRules := mergedColorPolicy.Spec.Proper().Rules
			if len(mergedRules) != len(tc.expected) {
				t.Errorf("Expected %d rules, but got %d", len(tc.expected), len(mergedRules))
			}
			for _, colorRule := range mergedRules {
				if tc.expected[colorRule.Id] != colorRule.Color {
					t.Errorf("Expected rule %s to have color %s, but got %s", colorRule.Id, tc.expected[colorRule.Id], colorRule.Color)
				}
			}
		})
	}
}
