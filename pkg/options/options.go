package options

//Debug is used to enable or disable the debug mode
var Debug *bool

// GetDebugMode is used to safely access the debug flag value
func GetDebugMode() bool {
	if Debug != nil && *Debug {
		return true
	}
	return false
}
