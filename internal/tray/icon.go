package tray

// embed is required for the go:embed directive that bundles the default tray
// icon into the binary.
import _ "embed"

//go:embed tray-icon.ico
var DefaultIcon []byte
