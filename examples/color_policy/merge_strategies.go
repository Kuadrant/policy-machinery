package color_policy

import (
	"github.com/samber/lo"

	"github.com/kuadrant/policy-machinery/machinery"
)

const (
	AtomicMergeStrategy     = "atomic"
	PolicyRuleMergeStrategy = "merge"
)

// AtomicDefaultsMergeStrategy implements a merge strategy that returns the target Policy if it exists,
// otherwise it returns the source Policy.
func AtomicDefaultsMergeStrategy(source, target machinery.Policy) machinery.Policy {
	if source == nil {
		return target
	}
	if target == nil {
		return source
	}

	targetColorPolicy := target.(*ColorPolicy)
	if len(targetColorPolicy.Spec.Proper().Rules) > 0 {
		return targetColorPolicy.DeepCopy()
	}
	return source.(*ColorPolicy).DeepCopy()
}

var _ machinery.MergeStrategy = AtomicDefaultsMergeStrategy

// AtomicOverridesMergeStrategy implements a merge strategy that overrides a target Policy with
// a source one.
func AtomicOverridesMergeStrategy(source, _ machinery.Policy) machinery.Policy {
	if source == nil {
		return nil
	}
	return source.(*ColorPolicy).DeepCopy()
}

var _ machinery.MergeStrategy = AtomicOverridesMergeStrategy

// PolicyRuleDefaultsMergeStrategy implements a merge strategy that merges a source Policy into a target one
// by keeping the policy rules from the target and adding the ones from the source that do not exist in the target.
func PolicyRuleDefaultsMergeStrategy(source, target machinery.Policy) machinery.Policy {
	if source == nil {
		return target
	}
	if target == nil {
		return source
	}

	sourceColorPolicy := source.(*ColorPolicy)
	targetColorPolicy := target.(*ColorPolicy)

	additionalSourceRules := lo.Reject(sourceColorPolicy.Spec.Proper().Rules, specContainsRuleFunc(targetColorPolicy.Spec))
	mergedPolicy := targetColorPolicy.DeepCopy()
	mergedPolicy.Spec.Proper().Rules = append(mergedPolicy.Spec.Proper().Rules, additionalSourceRules...)
	return mergedPolicy
}

var _ machinery.MergeStrategy = PolicyRuleDefaultsMergeStrategy

// PolicyRuleOverridesMergeStrategy implements a merge strategy that merges a source Policy into a target one
// by using the policy rules from the source and keeping from the target only the policy rules that do not exist in
// the source.
func PolicyRuleOverridesMergeStrategy(source, target machinery.Policy) machinery.Policy {
	sourceColorPolicy := source.(*ColorPolicy)
	targetColorPolicy := target.(*ColorPolicy)

	rules := lo.Map(targetColorPolicy.Spec.Proper().Rules, func(targetRule ColorRule, _ int) ColorRule {
		if sourceRule, found := lo.Find(sourceColorPolicy.Spec.Proper().Rules, equalRuleFunc(targetRule)); found {
			return sourceRule
		}
		return targetRule
	})
	additionalSourceRules := lo.Reject(sourceColorPolicy.Spec.Proper().Rules, specContainsRuleFunc(targetColorPolicy.Spec))
	mergedPolicy := targetColorPolicy.DeepCopy()
	mergedPolicy.Spec.Proper().Rules = append(rules, additionalSourceRules...)
	return mergedPolicy
}

var _ machinery.MergeStrategy = PolicyRuleOverridesMergeStrategy

func specContainsRuleFunc(spec ColorSpec) func(ColorRule, int) bool {
	return func(rule ColorRule, _ int) bool {
		return lo.ContainsBy(spec.Rules, equalRuleFunc(rule))
	}
}

func equalRuleFunc(rule ColorRule) func(ColorRule) bool {
	return func(other ColorRule) bool {
		return rule.Id == other.Id
	}
}

func defaultsMergeStrategy(strategy string) machinery.MergeStrategy {
	switch strategy {
	case AtomicMergeStrategy:
		return AtomicDefaultsMergeStrategy
	case PolicyRuleMergeStrategy:
		return PolicyRuleDefaultsMergeStrategy
	default:
		return AtomicDefaultsMergeStrategy
	}
}

func overridesMergeStrategy(strategy string) machinery.MergeStrategy {
	switch strategy {
	case AtomicMergeStrategy:
		return AtomicOverridesMergeStrategy
	case PolicyRuleMergeStrategy:
		return PolicyRuleOverridesMergeStrategy
	default:
		return AtomicOverridesMergeStrategy
	}
}
