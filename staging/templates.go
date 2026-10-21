// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

// Package staging provides CSE scripts shared with inline provisioning.
package staging

import _ "embed"

// WindowsSSHFunctions is shared by the CSE package and the inline CustomData scripts.
//
//go:embed cse/windows/sshfunc.ps1
var WindowsSSHFunctions []byte
