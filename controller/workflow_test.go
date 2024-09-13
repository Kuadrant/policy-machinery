package controller

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/samber/lo"

	"github.com/kuadrant/policy-machinery/machinery"
)

func TestWorkflow(t *testing.T) {
	reconcileFuncFor := func(flag *bool, err error) ReconcileFunc {
		return func(context.Context, []ResourceEvent, *machinery.Topology, error, *sync.Map) error {
			*flag = true
			return err
		}
	}

	var preconditionCalled, task1Called, task2Called, postconditionCalled, errorHandled bool

	precondition := reconcileFuncFor(&preconditionCalled, nil)
	preconditionWithError := reconcileFuncFor(&preconditionCalled, fmt.Errorf("precondition error"))
	task1 := reconcileFuncFor(&task1Called, nil)
	task1WithError := reconcileFuncFor(&task1Called, fmt.Errorf("task1 error"))
	task2 := reconcileFuncFor(&task2Called, nil)
	task2WithError := reconcileFuncFor(&task2Called, fmt.Errorf("task2 error"))
	postcondition := reconcileFuncFor(&postconditionCalled, nil)
	postconditionWithError := reconcileFuncFor(&postconditionCalled, fmt.Errorf("postcondition error"))

	handleErrorAndSupress := func(context.Context, []ResourceEvent, *machinery.Topology, error, *sync.Map) error {
		errorHandled = true
		return nil
	}

	handleErrorAndRaise := func(_ context.Context, _ []ResourceEvent, _ *machinery.Topology, err error, _ *sync.Map) error {
		errorHandled = true
		return err
	}

	testCases := []struct {
		name                        string
		workflow                    *Workflow
		expectedPreconditionCalled  bool
		expectedTask1Called         bool
		expectedTask2Called         bool
		expectedPostconditionCalled bool
		possibleErrs                []error
		expectedErrorHandled        bool
	}{
		{
			name:     "empty workflow",
			workflow: &Workflow{},
		},
		{
			name: "precondition",
			workflow: &Workflow{
				Precondition: precondition,
			},
			expectedPreconditionCalled: true,
		},
		{
			name: "precondition and tasks",
			workflow: &Workflow{
				Precondition: precondition,
				Tasks:        []ReconcileFunc{task1, task2},
			},
			expectedPreconditionCalled: true,
			expectedTask1Called:        true,
			expectedTask2Called:        true,
		},
		{
			name: "precondition with error",
			workflow: &Workflow{
				Precondition: preconditionWithError,
				Tasks:        []ReconcileFunc{task1, task2},
			},
			expectedPreconditionCalled: true,
			expectedTask1Called:        false,
			expectedTask2Called:        false,
			possibleErrs:               []error{fmt.Errorf("precondition error")},
		},
		{
			name: "task1 with error",
			workflow: &Workflow{
				Tasks:         []ReconcileFunc{task1WithError, task2},
				Postcondition: postcondition,
			},
			expectedTask1Called:        true,
			expectedTask2Called:        true,
			expectedPreconditionCalled: false,
			possibleErrs:               []error{fmt.Errorf("task1 error")},
		},
		{
			name: "task2 with error",
			workflow: &Workflow{
				Tasks:         []ReconcileFunc{task1, task2WithError},
				Postcondition: postcondition,
			},
			expectedTask1Called:        true,
			expectedTask2Called:        true,
			expectedPreconditionCalled: false,
			possibleErrs:               []error{fmt.Errorf("task2 error")},
		},
		{
			name: "task1 and task2 with error",
			workflow: &Workflow{
				Tasks:         []ReconcileFunc{task1WithError, task2WithError},
				Postcondition: postcondition,
			},
			expectedTask1Called:        true,
			expectedTask2Called:        true,
			expectedPreconditionCalled: false,
			possibleErrs: []error{
				fmt.Errorf("task1 error"),
				fmt.Errorf("task2 error"),
			},
		},
		{
			name: "postcondition",
			workflow: &Workflow{
				Precondition:  precondition,
				Tasks:         []ReconcileFunc{task1, task2},
				Postcondition: postcondition,
			},
			expectedPreconditionCalled:  true,
			expectedTask1Called:         true,
			expectedTask2Called:         true,
			expectedPostconditionCalled: true,
		},
		{
			name: "postconditions with error",
			workflow: &Workflow{
				Precondition:  precondition,
				Tasks:         []ReconcileFunc{task1, task2},
				Postcondition: postconditionWithError,
			},
			expectedPreconditionCalled:  true,
			expectedTask1Called:         true,
			expectedTask2Called:         true,
			expectedPostconditionCalled: true,
			possibleErrs:                []error{fmt.Errorf("postcondition error")},
		},
		{
			name: "handle error and suppress",
			workflow: &Workflow{
				Precondition:  preconditionWithError,
				Postcondition: postconditionWithError,
				ErrorHandler:  handleErrorAndSupress,
			},
			expectedPreconditionCalled:  true,
			expectedPostconditionCalled: true,
			expectedErrorHandled:        true,
		},
		{
			name: "handle error and raise",
			workflow: &Workflow{
				Precondition:  preconditionWithError,
				Postcondition: postconditionWithError,
				ErrorHandler:  handleErrorAndRaise,
			},
			expectedPreconditionCalled: true,
			expectedErrorHandled:       true,
			possibleErrs:               []error{fmt.Errorf("precondition error")},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// reset
			preconditionCalled = false
			task1Called = false
			task2Called = false
			postconditionCalled = false
			errorHandled = false

			err := tc.workflow.Run(context.Background(), nil, nil, nil, nil)
			possibleErrs := lo.Map(tc.possibleErrs, func(err error, _ int) string { return err.Error() })

			if tc.expectedPreconditionCalled != preconditionCalled {
				t.Errorf("expected precondition to be called: %t, got %t", tc.expectedPreconditionCalled, preconditionCalled)
			}
			if tc.expectedTask1Called != task1Called {
				t.Errorf("expected task1 to be called: %t, got %t", tc.expectedTask1Called, task1Called)
			}
			if tc.expectedTask2Called != task2Called {
				t.Errorf("expected task2 to be called: %t, got %t", tc.expectedTask2Called, task2Called)
			}
			if tc.expectedPostconditionCalled != postconditionCalled {
				t.Errorf("expected postcondition to be called: %t, got %t", tc.expectedPostconditionCalled, postconditionCalled)
			}
			if len(possibleErrs) > 0 && err == nil {
				t.Errorf("expected one of the following errors (%v), got nil", strings.Join(possibleErrs, " / "))
			}
			if len(possibleErrs) == 0 && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
			if len(possibleErrs) > 0 && err != nil && !lo.ContainsBy(possibleErrs, func(possibleErr string) bool { return possibleErr == err.Error() }) {
				t.Errorf("expected error of the following errors (%v), got %v", strings.Join(possibleErrs, " / "), err)
			}
			if tc.expectedErrorHandled != errorHandled {
				t.Errorf("expected error handler called: %t, got %t", tc.expectedErrorHandled, errorHandled)
			}
		})
	}

}
