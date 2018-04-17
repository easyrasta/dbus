//+build !windows,!solaris

package dbus


func getSystemBusPlatformAddress() string {
	address := os.Getenv("DBUS_SYSTEM_BUS_ADDRESS")
	if address != "" {
		return fmt.Sprintf("unix:path=%s", address)
	}
	return defaultSystemBusAddress
}