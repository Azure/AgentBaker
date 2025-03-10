// Package parts provides the embedded templates used in Linux and Windows systems.
package parts

import "embed"

// Templates is an embedded filesystem that contains the templates used in Linux and Windows systems.
//
//go:embed *
var Templates embed.FS
