//go:build unit

package json_patch

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
			name: "JSON patch defaults into empty policy",
			source: &ColorPolicy{
				Spec: ColorSpec{
					Defaults: &ColorSpecProper{
						Rules: map[string]ColorValue{
							"rule-1": Blue,
							"rule-2": Red,
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
			name: "JSON patch defaults into policy without conflicting rules",
			source: &ColorPolicy{
				Spec: ColorSpec{
					Defaults: &ColorSpecProper{
						Rules: map[string]ColorValue{
							"rule-1": Blue,
							"rule-2": Red,
						},
					},
				},
			},
			target: &ColorPolicy{
				Spec: ColorSpec{
					ColorSpecProper: ColorSpecProper{
						Rules: map[string]ColorValue{
							"rule-3": Green,
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
			name: "JSON patch defaults into policy with conflicting rules",
			source: &ColorPolicy{
				Spec: ColorSpec{
					Defaults: &ColorSpecProper{
						Rules: map[string]ColorValue{
							"rule-1": Blue,
							"rule-2": Red,
						},
					},
				},
			},
			target: &ColorPolicy{
				Spec: ColorSpec{
					ColorSpecProper: ColorSpecProper{
						Rules: map[string]ColorValue{
							"rule-1": Yellow,
							"rule-3": Green,
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
			name: "JSON patch overrides into empty policy",
			source: &ColorPolicy{
				Spec: ColorSpec{
					Overrides: &ColorSpecProper{
						Rules: map[string]ColorValue{
							"rule-1": Blue,
							"rule-2": Red,
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
			name: "JSON patch overrides into policy without conflicting rules",
			source: &ColorPolicy{
				Spec: ColorSpec{
					Overrides: &ColorSpecProper{
						Rules: map[string]ColorValue{
							"rule-1": Blue,
							"rule-2": Red,
						},
					},
				},
			},
			target: &ColorPolicy{
				Spec: ColorSpec{
					ColorSpecProper: ColorSpecProper{
						Rules: map[string]ColorValue{
							"rule-3": Green,
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
			name: "JSON patch overrides into policy with conflicting rules",
			source: &ColorPolicy{
				Spec: ColorSpec{
					Overrides: &ColorSpecProper{
						Rules: map[string]ColorValue{
							"rule-1": Blue,
							"rule-2": Red,
						},
					},
				},
			},
			target: &ColorPolicy{
				Spec: ColorSpec{
					ColorSpecProper: ColorSpecProper{
						Rules: map[string]ColorValue{
							"rule-1": Yellow,
							"rule-3": Green,
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
			for id, color := range mergedRules {
				if tc.expected[id] != color {
					t.Errorf("Expected rule %s to have color %s, but got %s", id, tc.expected[id], color)
				}
			}
		})
	}
}
