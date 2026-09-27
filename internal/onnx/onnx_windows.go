//go:build windows

// Package onnx runs ONNX models with ONNX Runtime (onnxruntime.dll) through its C API,
// called directly with syscall (no cgo). Only what the app needs: one float tensor in,
// the first float tensor out.
package onnx

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Indices of the functions used, in the OrtApi table of onnxruntime_c_api.h (the table only
// grows at the end, so they are the same in every version since 1.0).
const (
	fnGetErrorMessage                  = 2
	fnCreateEnv                        = 3
	fnCreateSession                    = 7
	fnRun                              = 9
	fnCreateSessionOptions             = 10
	fnSetSessionGraphOptimizationLevel = 23
	fnSetIntraOpNumThreads             = 24
	fnSessionGetInputName              = 36
	fnSessionGetOutputName             = 37
	fnCreateTensorWithDataAsOrtValue   = 49
	fnGetTensorMutableData             = 51
	fnGetDimensionsCount               = 61
	fnGetDimensions                    = 62
	fnGetTensorTypeAndShape            = 65
	fnCreateCpuMemoryInfo              = 69
	fnAllocatorFree                    = 76
	fnGetAllocatorWithDefaultOptions   = 78
	fnReleaseEnv                       = 92
	fnReleaseStatus                    = 93
	fnReleaseMemoryInfo                = 94
	fnReleaseSession                   = 95
	fnReleaseValue                     = 96
	fnReleaseTensorTypeAndShapeInfo    = 99
	fnReleaseSessionOptions            = 100
)

// apiVersion is the oldest C API version asked for (ONNX Runtime 1.16).
const apiVersion = 16

// Runtime is a loaded onnxruntime.dll with one environment.
type Runtime struct {
	api uintptr // *OrtApi
	env uintptr // *OrtEnv
}

var (
	loadMu sync.Mutex
	loaded = map[string]*Runtime{}
)

// Load loads onnxruntime.dll (once per path).
func Load(dll string) (*Runtime, error) {
	loadMu.Lock()
	defer loadMu.Unlock()
	if rt := loaded[dll]; rt != nil {
		return rt, nil
	}
	h, err := windows.LoadLibraryEx(dll, 0, windows.LOAD_WITH_ALTERED_SEARCH_PATH)
	if err != nil {
		return nil, fmt.Errorf("onnxruntime.dll: %w", err)
	}
	getBase, err := windows.GetProcAddress(h, "OrtGetApiBase")
	if err != nil {
		return nil, fmt.Errorf("OrtGetApiBase: %w", err)
	}
	base, _, _ := syscall.SyscallN(getBase)
	if base == 0 {
		return nil, errors.New("OrtGetApiBase returned nothing")
	}
	// OrtApiBase { GetApi(uint32) *OrtApi; GetVersionString() *char }
	getAPI := *(*uintptr)(unsafe.Pointer(base))
	api, _, _ := syscall.SyscallN(getAPI, apiVersion)
	if api == 0 {
		return nil, errors.New("onnxruntime.dll is too old")
	}
	rt := &Runtime{api: api}
	logid := cstr("kmb")
	var env uintptr
	if err := rt.call(fnCreateEnv, 3 /* ORT_LOGGING_LEVEL_ERROR */, ptr(logid), uintptr(unsafe.Pointer(&env))); err != nil {
		return nil, err
	}
	runtime.KeepAlive(logid)
	rt.env = env
	loaded[dll] = rt
	return rt, nil
}

// Version returns the version string of the loaded library.
func Version(dll string) (string, error) {
	h, err := windows.LoadLibraryEx(dll, 0, windows.LOAD_WITH_ALTERED_SEARCH_PATH)
	if err != nil {
		return "", err
	}
	getBase, err := windows.GetProcAddress(h, "OrtGetApiBase")
	if err != nil {
		return "", err
	}
	base, _, _ := syscall.SyscallN(getBase)
	if base == 0 {
		return "", errors.New("no api base")
	}
	getVersion := *(*uintptr)(unsafe.Pointer(base + unsafe.Sizeof(uintptr(0))))
	v, _, _ := syscall.SyscallN(getVersion)
	return gostr(v), nil
}

func (rt *Runtime) fn(i int) uintptr {
	return *(*uintptr)(unsafe.Pointer(rt.api + uintptr(i)*unsafe.Sizeof(uintptr(0))))
}

// call runs an OrtApi function that returns an OrtStatus* (nil = success).
func (rt *Runtime) call(i int, args ...uintptr) error {
	st, _, _ := syscall.SyscallN(rt.fn(i), args...)
	if st == 0 {
		return nil
	}
	msg, _, _ := syscall.SyscallN(rt.fn(fnGetErrorMessage), st)
	text := gostr(msg)
	syscall.SyscallN(rt.fn(fnReleaseStatus), st)
	return errors.New("onnxruntime: " + text)
}

func (rt *Runtime) release(i int, p uintptr) {
	if p != 0 {
		syscall.SyscallN(rt.fn(i), p)
	}
}

// Session is a loaded model.
type Session struct {
	rt      *Runtime
	s       uintptr
	in, out []byte // C strings of the first input and output name
	mu      sync.Mutex
}

// Open loads a model file; threads 0 lets ONNX Runtime choose.
func (rt *Runtime) Open(model string, threads int) (*Session, error) {
	var opts uintptr
	if err := rt.call(fnCreateSessionOptions, uintptr(unsafe.Pointer(&opts))); err != nil {
		return nil, err
	}
	defer rt.release(fnReleaseSessionOptions, opts)
	if threads > 0 {
		if err := rt.call(fnSetIntraOpNumThreads, opts, uintptr(threads)); err != nil {
			return nil, err
		}
	}
	if err := rt.call(fnSetSessionGraphOptimizationLevel, opts, 99 /* ORT_ENABLE_ALL */); err != nil {
		return nil, err
	}
	path, err := windows.UTF16PtrFromString(model)
	if err != nil {
		return nil, err
	}
	var s uintptr
	if err := rt.call(fnCreateSession, rt.env, uintptr(unsafe.Pointer(path)), opts, uintptr(unsafe.Pointer(&s))); err != nil {
		return nil, err
	}
	sess := &Session{rt: rt, s: s}
	var alloc uintptr
	if err := rt.call(fnGetAllocatorWithDefaultOptions, uintptr(unsafe.Pointer(&alloc))); err != nil {
		sess.Close()
		return nil, err
	}
	name := func(fn int) ([]byte, error) {
		var p uintptr
		if err := rt.call(fn, s, 0, alloc, uintptr(unsafe.Pointer(&p))); err != nil {
			return nil, err
		}
		n := cstr(gostr(p))
		rt.call(fnAllocatorFree, alloc, p)
		return n, nil
	}
	if sess.in, err = name(fnSessionGetInputName); err != nil {
		sess.Close()
		return nil, err
	}
	if sess.out, err = name(fnSessionGetOutputName); err != nil {
		sess.Close()
		return nil, err
	}
	return sess, nil
}

// Close frees the model.
func (s *Session) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rt.release(fnReleaseSession, s.s)
	s.s = 0
}

// Run feeds one float32 tensor of the given shape and returns the first output and its shape.
func (s *Session) Run(input []float32, shape []int64) ([]float32, []int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.s == 0 {
		return nil, nil, errors.New("session closed")
	}
	rt := s.rt
	n := int64(1)
	for _, d := range shape {
		n *= d
	}
	if n != int64(len(input)) || n == 0 {
		return nil, nil, errors.New("tensor size mismatch")
	}
	var mem uintptr
	if err := rt.call(fnCreateCpuMemoryInfo, 1 /* OrtArenaAllocator */, 0 /* OrtMemTypeDefault */, uintptr(unsafe.Pointer(&mem))); err != nil {
		return nil, nil, err
	}
	defer rt.release(fnReleaseMemoryInfo, mem)
	var in uintptr
	if err := rt.call(fnCreateTensorWithDataAsOrtValue, mem, uintptr(unsafe.Pointer(&input[0])), uintptr(len(input)*4),
		uintptr(unsafe.Pointer(&shape[0])), uintptr(len(shape)), 1 /* FLOAT */, uintptr(unsafe.Pointer(&in))); err != nil {
		return nil, nil, err
	}
	defer rt.release(fnReleaseValue, in)
	inName, outName := ptr(s.in), ptr(s.out)
	var out uintptr
	err := rt.call(fnRun, s.s, 0, uintptr(unsafe.Pointer(&inName)), uintptr(unsafe.Pointer(&in)), 1,
		uintptr(unsafe.Pointer(&outName)), 1, uintptr(unsafe.Pointer(&out)))
	runtime.KeepAlive(input)
	runtime.KeepAlive(shape)
	runtime.KeepAlive(s.in)
	runtime.KeepAlive(s.out)
	if err != nil {
		return nil, nil, err
	}
	defer rt.release(fnReleaseValue, out)
	var info uintptr
	if err := rt.call(fnGetTensorTypeAndShape, out, uintptr(unsafe.Pointer(&info))); err != nil {
		return nil, nil, err
	}
	defer rt.release(fnReleaseTensorTypeAndShapeInfo, info)
	var dims uintptr
	if err := rt.call(fnGetDimensionsCount, info, uintptr(unsafe.Pointer(&dims))); err != nil {
		return nil, nil, err
	}
	outShape := make([]int64, dims)
	if dims > 0 {
		if err := rt.call(fnGetDimensions, info, uintptr(unsafe.Pointer(&outShape[0])), dims); err != nil {
			return nil, nil, err
		}
	}
	count := int64(1)
	for _, d := range outShape {
		count *= d
	}
	var data uintptr
	if err := rt.call(fnGetTensorMutableData, out, uintptr(unsafe.Pointer(&data))); err != nil {
		return nil, nil, err
	}
	res := make([]float32, count)
	copy(res, unsafe.Slice((*float32)(unsafe.Pointer(data)), count))
	return res, outShape, nil
}

func cstr(s string) []byte { return append([]byte(s), 0) }

func ptr(b []byte) uintptr { return uintptr(unsafe.Pointer(&b[0])) }

// gostr copies a NUL-terminated C string.
func gostr(p uintptr) string {
	if p == 0 {
		return ""
	}
	var b []byte
	for i := uintptr(0); i < 4096; i++ {
		c := *(*byte)(unsafe.Pointer(p + i))
		if c == 0 {
			break
		}
		b = append(b, c)
	}
	return string(b)
}
