package audio

/*
#cgo LDFLAGS: -L${SRCDIR}/.. -llivekit_ffi -Wl,-rpath,${SRCDIR}/..
#include <stdlib.h>
#include <stdint.h>
#include <stdbool.h>

typedef uint64_t FfiHandleId;

FfiHandleId livekit_ffi_request(const uint8_t *data, size_t len,
                               const uint8_t **res_ptr, size_t *res_len);

bool livekit_ffi_drop_handle(FfiHandleId handle_id);
*/
import "C"
import (
	"fmt"
	"unsafe"

	"google.golang.org/protobuf/proto"
)

// FfiClient provides the interface to the LiveKit FFI library
type FfiClient struct{}

// NewFfiClient creates a new FFI client
func NewFfiClient() *FfiClient {
	return &FfiClient{}
}

// Request sends a protobuf request to the FFI library and returns the response
func (c *FfiClient) Request(req proto.Message) ([]byte, error) {
	// Serialize the request
	reqData, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Prepare C variables
	var resPtr *C.uint8_t
	var resLen C.size_t

	// Call the FFI function
	handle := C.livekit_ffi_request(
		(*C.uint8_t)(C.CBytes(reqData)),
		C.size_t(len(reqData)),
		&resPtr,
		&resLen,
	)

	if handle == 0 {
		return nil, fmt.Errorf("FFI request failed, returned invalid handle")
	}

	// Convert C response to Go bytes
	if resPtr == nil || resLen == 0 {
		return nil, fmt.Errorf("FFI returned null or empty response")
	}

	responseData := C.GoBytes(unsafe.Pointer(resPtr), C.int(resLen))
	
	// Note: We don't need to drop the handle for request/response pattern
	// The handle is used internally by the FFI library for the response
	
	return responseData, nil
}

// DropHandle releases a handle in the FFI library
func (c *FfiClient) DropHandle(handleId uint64) error {
	success := C.livekit_ffi_drop_handle(C.FfiHandleId(handleId))
	if !success {
		return fmt.Errorf("failed to drop handle %d", handleId)
	}
	return nil
}