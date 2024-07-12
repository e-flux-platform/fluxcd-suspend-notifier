package notification

import (
	"context"
	"fmt"

	"github.com/expr-lang/expr"
	_ "github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

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

type FilteringNotifier struct {
	filter   *vm.Program
	delegate Notifier
}

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
