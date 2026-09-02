package app

import (
	"os"
	"strconv"

	"golang.org/x/sys/windows/registry"
)

// Lancement automatique au demarrage de Windows : une simple valeur dans la
// base de registre de l'utilisateur, sans droits administrateur.

const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`

func startupEnabled() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()
	value, _, err := key.GetStringValue(appName)
	return err == nil && value != ""
}

// setStartup active ou desactive le lancement automatique et renvoie l'etat
// reellement obtenu.
func setStartup(enabled bool) bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return startupEnabled()
	}
	defer key.Close()

	if !enabled {
		_ = key.DeleteValue(appName)
		return false
	}
	executable, err := os.Executable()
	if err != nil {
		return false
	}
	if err := key.SetStringValue(appName, strconv.Quote(executable)); err != nil {
		return false
	}
	return true
}
