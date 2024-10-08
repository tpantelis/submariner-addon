// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "github.com/stolostron/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/listers"
	"k8s.io/client-go/tools/cache"
)

// SubmarinerConfigLister helps list SubmarinerConfigs.
// All objects returned here must be treated as read-only.
type SubmarinerConfigLister interface {
	// List lists all SubmarinerConfigs in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.SubmarinerConfig, err error)
	// SubmarinerConfigs returns an object that can list and get SubmarinerConfigs.
	SubmarinerConfigs(namespace string) SubmarinerConfigNamespaceLister
	SubmarinerConfigListerExpansion
}

// submarinerConfigLister implements the SubmarinerConfigLister interface.
type submarinerConfigLister struct {
	listers.ResourceIndexer[*v1alpha1.SubmarinerConfig]
}

// NewSubmarinerConfigLister returns a new SubmarinerConfigLister.
func NewSubmarinerConfigLister(indexer cache.Indexer) SubmarinerConfigLister {
	return &submarinerConfigLister{listers.New[*v1alpha1.SubmarinerConfig](indexer, v1alpha1.Resource("submarinerconfig"))}
}

// SubmarinerConfigs returns an object that can list and get SubmarinerConfigs.
func (s *submarinerConfigLister) SubmarinerConfigs(namespace string) SubmarinerConfigNamespaceLister {
	return submarinerConfigNamespaceLister{listers.NewNamespaced[*v1alpha1.SubmarinerConfig](s.ResourceIndexer, namespace)}
}

// SubmarinerConfigNamespaceLister helps list and get SubmarinerConfigs.
// All objects returned here must be treated as read-only.
type SubmarinerConfigNamespaceLister interface {
	// List lists all SubmarinerConfigs in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.SubmarinerConfig, err error)
	// Get retrieves the SubmarinerConfig from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha1.SubmarinerConfig, error)
	SubmarinerConfigNamespaceListerExpansion
}

// submarinerConfigNamespaceLister implements the SubmarinerConfigNamespaceLister
// interface.
type submarinerConfigNamespaceLister struct {
	listers.ResourceIndexer[*v1alpha1.SubmarinerConfig]
}
