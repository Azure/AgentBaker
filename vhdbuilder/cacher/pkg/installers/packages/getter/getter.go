package getter

import "github.com/Azure/agentbaker/vhdbuilder/cacher/pkg/model"

type Getter interface {
	Get(pkg *model.Package) error
}
