package notification

import (
	"context"
	"fmt"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

// NewFilteringNotifier instantiates and returns FilteringNotifier
func NewFilteringNotifier(rawExpr string, delegate Notifier) (*FilteringNotifier, error) {
	filter, err := expr.Compile(rawExpr)
	if err != nil {
		return nil, fmt.Errorf("failed to compile filter expression: %w", err)
	}
	return &FilteringNotifier{
		filter:   filter,
		delegate: delegate,
	}, nil
}

// FilteringNotifier is a notifier implementation that filters notifications via an expression.
type FilteringNotifier struct {
	filter   *vm.Program
	delegate Notifier
}

// Notify passes the notification to the underlying delegate if the expression is satisfied.
func (fn *FilteringNotifier) Notify(ctx context.Context, notif Notification) error {
	env := map[string]interface{}{
		"resource":  notif.Resource,
		"suspended": notif.Suspended,
		"email":     notif.Email,
	}

	output, err := expr.Run(fn.filter, env)
	if err != nil {
		return fmt.Errorf("failed to evaluate expression: %w", err)
	}

	include, ok := output.(bool)
	if !ok {
		return fmt.Errorf("expression evaluated to %v, but was not a boolean", output)
	}

	if !include {
		return nil
	}
	return fn.delegate.Notify(ctx, notif)
}
