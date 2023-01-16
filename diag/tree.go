package diag

import (
	"context"
	"fmt"

	"github.com/cschleiden/go-workflows/internal/core"
	"github.com/cschleiden/go-workflows/internal/history"
)

type instanceTreeBuilder struct {
	b            Backend
	instanceByID map[string]*WorkflowInstanceRef
}

func NewInstanceTreeBuilder(db Backend) *instanceTreeBuilder {
	return &instanceTreeBuilder{
		b:            db,
		instanceByID: map[string]*WorkflowInstanceRef{},
	}
}

func (itb *instanceTreeBuilder) BuildWorkflowInstanceTree(ctx context.Context, instanceID string) (*WorkflowInstanceTree, error) {
	instance, err := itb.getInstance(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("getting instance: %w", err)
	}

	// Get root instance of tree
	rootInstance, err := itb.getRoot(ctx, instance)
	if err != nil {
		return nil, fmt.Errorf("getting root instance: %w", err)
	}

	if rootInstance == nil {
		return nil, fmt.Errorf("no root instance found")
	}

	root := &WorkflowInstanceTree{
		WorkflowInstanceRef: rootInstance,
		Children:            []*WorkflowInstanceTree{},
	}

	s := []*WorkflowInstanceTree{root}
	for len(s) > 0 {
		node := s[0]
		s = s[1:]

		name, children, err := itb.getNameAndChildren(ctx, node.Instance)
		if err != nil {
			return nil, fmt.Errorf("getting children of instance %s: %w", node.Instance.InstanceID, err)
		}

		node.WorkflowName = name

		for _, child := range children {
			t := &WorkflowInstanceTree{
				WorkflowInstanceRef: child,
				Children:            []*WorkflowInstanceTree{},
			}

			// Enqueue
			s = append(s, t)

			// Add to current node
			node.Children = append(node.Children, t)
		}
	}

	return root, nil
}

func (itb *instanceTreeBuilder) getRoot(ctx context.Context, instance *WorkflowInstanceRef) (*WorkflowInstanceRef, error) {
	parentInstanceID := instance.Instance.ParentInstanceID
	for parentInstanceID != "" {
		var err error
		instance, err = itb.getInstance(ctx, parentInstanceID)
		if err != nil {
			return nil, err
		}

		parentInstanceID = instance.Instance.ParentInstanceID
	}

	return instance, nil
}

func (itb *instanceTreeBuilder) getNameAndChildren(ctx context.Context, instance *core.WorkflowInstance) (string, []*WorkflowInstanceRef, error) {
	h, err := itb.b.GetWorkflowInstanceHistory(ctx, instance, nil)
	if err != nil {
		return "", nil, fmt.Errorf("getting instance history: %w", err)
	}

	workflowName := ""

	var children []*WorkflowInstanceRef
	for _, event := range h {
		switch event.Type {
		case history.EventType_SubWorkflowScheduled:
			childInstance, err := itb.getInstance(ctx, event.Attributes.(*history.SubWorkflowScheduledAttributes).SubWorkflowInstance.InstanceID)
			if err != nil {
				return "", nil, fmt.Errorf("getting child instance: %w", err)
			}

			children = append(children, childInstance)

		case history.EventType_WorkflowExecutionStarted:
			workflowName = event.Attributes.(*history.ExecutionStartedAttributes).Name
		}
	}

	return workflowName, children, nil
}

func (itb *instanceTreeBuilder) getInstance(ctx context.Context, instanceID string) (*WorkflowInstanceRef, error) {
	if instance, ok := itb.instanceByID[instanceID]; ok {
		return instance, nil
	}

	instance, err := itb.b.GetWorkflowInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}

	itb.instanceByID[instanceID] = instance

	return instance, nil
}
