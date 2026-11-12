// Package parts provides the embedded templates used in Linux and Windows systems.
package parts

import "embed"

// Templates is an embedded filesystem that contains the templates used in Linux and Windows systems.
//
//go:embed *
var Templates embed.FS

// Root lets ANC embed the script trees without the unrelated service templates.
//
//go:embed root
var Root embed.FS
