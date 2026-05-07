//go:build darwin

package launchatlogin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc -mmacosx-version-min=11.0
#cgo LDFLAGS: -framework ServiceManagement -framework Foundation

#import <Foundation/Foundation.h>
#import <ServiceManagement/ServiceManagement.h>

// genie_login_status returns:
//   -1  unsupported (macOS < 13)
//    0  disabled / not registered
//    1  enabled
//    2  registered but waiting for user approval in System Settings
static int genie_login_status(void) {
    if (@available(macOS 13.0, *)) {
        SMAppService *svc = [SMAppService mainAppService];
        switch (svc.status) {
            case SMAppServiceStatusEnabled:          return 1;
            case SMAppServiceStatusRequiresApproval: return 2;
            default:                                 return 0;
        }
    }
    return -1;
}

// genie_login_set returns 1 on success, 0 on failure, -1 if unsupported.
// errOut (if non-NULL) receives a strdup'd UTF-8 error string the caller must free.
static int genie_login_set(int enable, char **errOut) {
    if (@available(macOS 13.0, *)) {
        SMAppService *svc = [SMAppService mainAppService];
        NSError *err = nil;
        BOOL ok = enable ? [svc registerAndReturnError:&err]
                         : [svc unregisterAndReturnError:&err];
        if (!ok && errOut != NULL && err != nil) {
            *errOut = strdup([[err localizedDescription] UTF8String]);
        }
        return ok ? 1 : 0;
    }
    return -1;
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

// Get reports the current launch-at-login state.
func Get() State {
	switch C.genie_login_status() {
	case 1:
		return State{Enabled: true, Supported: true}
	case 2:
		return State{Enabled: true, Supported: true, RequiresApproval: true}
	case 0:
		return State{Enabled: false, Supported: true}
	default:
		return State{Supported: false}
	}
}

// Set enables or disables launch-at-login.
func Set(enable bool) error {
	var cErr *C.char
	flag := C.int(0)
	if enable {
		flag = 1
	}
	rc := C.genie_login_set(flag, &cErr)
	if cErr != nil {
		defer C.free(unsafe.Pointer(cErr))
	}
	switch rc {
	case 1:
		return nil
	case -1:
		return errors.New("launch-at-login requires macOS 13 or newer")
	default:
		if cErr != nil {
			return errors.New(C.GoString(cErr))
		}
		return errors.New("failed to update login item")
	}
}
