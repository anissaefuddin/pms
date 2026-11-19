package items

import "context"

// Attribution: a change made by an automation rule is recorded as the Automation Bot acting for the rule. Who may make it is still
// decided by the rule's owner, so this only changes what the history says.
type attributionKey struct{}

type attribution struct{ ruleID string }

// WithAutomationRule marks ctx as a change made by the given rule.
func WithAutomationRule(ctx context.Context, ruleID string) context.Context {
	return context.WithValue(ctx, attributionKey{}, attribution{ruleID: ruleID})
}

// AttributionFrom returns the via label and rule id for ctx; both are empty and nil for a person's own change.
func AttributionFrom(ctx context.Context) (string, *string) {
	a, ok := ctx.Value(attributionKey{}).(attribution)
	if !ok {
		return "", nil
	}
	return "automation_bot", &a.ruleID
}
