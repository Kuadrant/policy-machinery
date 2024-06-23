package machinery

import "github.com/samber/lo"

var (
	_ MergeStrategy = AtomicDefaultsMergeStrategy
	_ MergeStrategy = AtomicOverridesMergeStrategy
	_ MergeStrategy = PolicyRuleDefaultsMergeStrategy
	_ MergeStrategy = PolicyRuleOverridesMergeStrategy
)

// AtomicDefaultsMergeStrategy implements a merge strategy that uses the target PolicySpec if it exists,
// otherwise it returns the source PolicySpec.
func AtomicDefaultsMergeStrategy(source, target PolicySpec) PolicySpec {
	if target != nil && len(target.GetRules()) > 0 {
		return target.DeepCopy()
	}
	return source.DeepCopy()
}

// AtomicOverridesMergeStrategy implements a merge strategy that overrides a target PolicySpec with
// a source one.
func AtomicOverridesMergeStrategy(source, _ PolicySpec) PolicySpec {
	return source.DeepCopy()
}

// PolicyRuleDefaultsMergeStrategy implements a merge strategy that merges a source PolicySpec into a target one
// by keeping the policy rules from the target and adding the ones from the source that do not exist in the target.
func PolicyRuleDefaultsMergeStrategy(source, target PolicySpec) PolicySpec {
	additionalSourceRules := lo.Reject(source.GetRules(), specContainsRuleFunc(target))
	newSpec := target.DeepCopy()
	newSpec.SetRules(append(newSpec.GetRules(), additionalSourceRules...))
	return newSpec
}

// PolicyRuleOverridesMergeStrategy implements a merge strategy that merges a source PolicySpec into a target one
// by using the policy rules from the source and keeping from the target only the policy rules that do not exist in
// the source.
func PolicyRuleOverridesMergeStrategy(source, target PolicySpec) PolicySpec {
	rules := lo.Map(target.GetRules(), func(rule Rule, _ int) Rule {
		if aRule, found := lo.Find(source.GetRules(), equalRuleFunc(rule)); found {
			return aRule
		}
		return rule
	})
	additionalSourceRules := lo.Reject(source.GetRules(), specContainsRuleFunc(target))
	newSpec := target.DeepCopy()
	newSpec.SetRules(append(rules, additionalSourceRules...))
	return newSpec
}

// Merge is a helper function that merges two PolicySpecs using a given MergeStrategy.
func Merge(source, target PolicySpec, strategy MergeStrategy) PolicySpec {
	if target == nil {
		return source
	}
	if source == nil {
		return target
	}
	return strategy(source, target)
}

func specContainsRuleFunc(spec PolicySpec) func(Rule, int) bool {
	return func(rule Rule, _ int) bool {
		return lo.ContainsBy(spec.GetRules(), equalRuleFunc(rule))
	}
}

func equalRuleFunc(rule Rule) func(Rule) bool {
	return func(other Rule) bool {
		return rule.GetId() == other.GetId()
	}
}
