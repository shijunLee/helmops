package actions

import (
	"context"

	helmactions "helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/release"
)

type ListOptions struct {
	Namespace         string
	KubernetesOptions *KubernetesClient
	// All ignores the limit/offset
	All bool
	// AllNamespaces searches across namespaces
	AllNamespaces bool
	// Sort indicates the sort to use
	//
	// see pkg/releaseutil for several useful sorters
	Sort helmactions.Sorter
	// Overrides the default lexicographic sorting
	ByDate      bool
	SortReverse bool
	// StateMask accepts a bitmask of states for items to show.
	// The default is ListDeployed
	StateMask helmactions.ListStates
	// Limit is the number of items to return per Run()
	Limit int
	// Offset is the starting index for the Run() call
	Offset int
	// Filter is a filter that is applied to the results
	Filter       string
	Short        bool
	TimeFormat   string
	Uninstalled  bool
	Superseded   bool
	Uninstalling bool
	Deployed     bool
	Failed       bool
	Pending      bool
	Selector     string
}

func (l *ListOptions) ListRelease(ctx context.Context, namespace string) ([]*release.Release, error) {
	if namespace != "" {
		l.Namespace = namespace
	}
	cfg, err := l.KubernetesOptions.GetHelmActionConfiguration(l.Namespace)
	if err != nil {
		return nil, err
	}
	listActions := helmactions.NewList(cfg)
	listActions.AllNamespaces = l.AllNamespaces
	listActions.All = l.All
	listActions.Sort = l.Sort
	listActions.ByDate = l.ByDate
	listActions.SortReverse = l.SortReverse
	listActions.StateMask = l.StateMask
	listActions.Limit = l.Limit
	listActions.Offset = l.Offset
	listActions.Filter = l.Filter
	listActions.Short = l.Short
	listActions.TimeFormat = l.TimeFormat
	listActions.Uninstalled = l.Uninstalled
	listActions.Superseded = l.Superseded
	listActions.Uninstalling = l.Uninstalling
	listActions.Deployed = l.Deployed
	listActions.Failed = l.Failed
	listActions.Pending = l.Pending
	listActions.Selector = l.Selector

	return listActions.Run()
}
