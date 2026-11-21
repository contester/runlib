package service

import (
	"errors"
	"fmt"
	"strings"

	"golang.org/x/sys/windows/registry"

	log "github.com/sirupsen/logrus"
)

const (
	regPathWER     = `SOFTWARE\Microsoft\Windows\Windows Error Reporting`
	regPathWinCtrl = `SYSTEM\CurrentControlSet\Control\Windows`
	regPathAeDebug = `SOFTWARE\Microsoft\Windows NT\CurrentVersion\AeDebug`
)

type regView struct {
	flag  uint32
	label string
}

var softwareRegViews = []regView{
	{flag: registry.WOW64_64KEY, label: "64-bit"},
	{flag: registry.WOW64_32KEY, label: "WOW64"},
}

// configureCrashDialogSuppression ensures HKLM crash-dialog suppression keys
// (WER, csrss hard-error mode, and AeDebug JIT prompts) are configured cleanly
// at runner startup. Checks before writing so already-configured hosts need no
// write access, and logs failures instead of failing startup.
func configureCrashDialogSuppression(disabled bool) {
	if disabled {
		log.Info("Crash-dialog registry setup disabled by config (disable_crash_dialog_registry_setup)")
		return
	}

	var updated []string
	var errs []string
	errorModeChanged := false

	record := func(desc string, changed bool, err error) bool {
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", desc, err))
			return false
		}
		if changed {
			updated = append(updated, desc)
		}
		return changed
	}

	for _, v := range softwareRegViews {
		changed, err := ensureRegDword(registry.LOCAL_MACHINE, regPathWER, v.flag, "DontShowUI", 1)
		record(fmt.Sprintf(`HKLM\%s\DontShowUI (%s)`, regPathWER, v.label), changed, err)

		changed, err = ensureRegDword(registry.LOCAL_MACHINE, regPathWER, v.flag, "Disabled", 1)
		record(fmt.Sprintf(`HKLM\%s\Disabled (%s)`, regPathWER, v.label), changed, err)
	}

	changed, err := ensureRegDword(registry.LOCAL_MACHINE, regPathWinCtrl, 0, "ErrorMode", 2)
	if record(fmt.Sprintf(`HKLM\%s\ErrorMode`, regPathWinCtrl), changed, err) {
		errorModeChanged = true
	}

	for _, v := range softwareRegViews {
		changed, err := ensureRegString(registry.LOCAL_MACHINE, regPathAeDebug, v.flag, "Auto", "0")
		record(fmt.Sprintf(`HKLM\%s\Auto (%s)`, regPathAeDebug, v.label), changed, err)

		changed, err = ensureRegValueDeleted(registry.LOCAL_MACHINE, regPathAeDebug, v.flag, "Debugger")
		record(fmt.Sprintf(`HKLM\%s\Debugger (%s)`, regPathAeDebug, v.label), changed, err)
	}

	if len(updated) > 0 {
		log.Infof("Updated crash-dialog suppression registry settings: %s", strings.Join(updated, ", "))
	}
	if errorModeChanged {
		log.Warn(`Updated HKLM\SYSTEM\CurrentControlSet\Control\Windows\ErrorMode to 2; reboot required for csrss.exe hard-error suppression to take effect`)
	}
	if len(errs) > 0 {
		log.Warnf("Failed to update some crash-dialog suppression registry settings (run runner as Administrator or import doc/disable-crash-dialogs.reg): %s", strings.Join(errs, "; "))
	}
	if len(updated) == 0 && len(errs) == 0 {
		log.Info("Crash-dialog suppression registry settings already configured")
	}
}

func ensureRegValue(root registry.Key, path string, view uint32, name string, matches func(registry.Key) bool, write func(registry.Key) error) (bool, error) {
	if k, err := registry.OpenKey(root, path, registry.QUERY_VALUE|view); err == nil {
		ok := matches(k)
		k.Close()
		if ok {
			return false, nil
		}
	}
	k, _, err := registry.CreateKey(root, path, registry.SET_VALUE|view)
	if err != nil {
		return false, err
	}
	defer k.Close()
	if err := write(k); err != nil {
		return false, err
	}
	return true, nil
}

func ensureRegDword(root registry.Key, path string, view uint32, name string, want uint32) (bool, error) {
	return ensureRegValue(
		root, path, view, name,
		func(k registry.Key) bool {
			val, valType, err := k.GetIntegerValue(name)
			return err == nil && valType == registry.DWORD && uint32(val) == want
		},
		func(k registry.Key) error {
			return k.SetDWordValue(name, want)
		},
	)
}

func ensureRegString(root registry.Key, path string, view uint32, name, want string) (bool, error) {
	return ensureRegValue(
		root, path, view, name,
		func(k registry.Key) bool {
			val, valType, err := k.GetStringValue(name)
			return err == nil && valType == registry.SZ && val == want
		},
		func(k registry.Key) error {
			return k.SetStringValue(name, want)
		},
	)
}

func ensureRegValueDeleted(root registry.Key, path string, view uint32, name string) (bool, error) {
	k, err := registry.OpenKey(root, path, registry.QUERY_VALUE|view)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	if err == nil {
		_, _, valErr := k.GetValue(name, nil)
		k.Close()
		if errors.Is(valErr, registry.ErrNotExist) {
			return false, nil
		}
	}

	wk, err := registry.OpenKey(root, path, registry.SET_VALUE|view)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer wk.Close()

	if err := wk.DeleteValue(name); errors.Is(err, registry.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}
