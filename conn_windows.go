//+build windows
package dbus

import "os"

func getSystemBusPlatformAddress() string {
	address := os.Getenv("DBUS_SYSTEM_BUS_ADDRESS")
	if address != "" {
		return address
	}
	return defaultSystemBusAddress
}
