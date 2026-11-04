//go:build windows

package main

// Minimal WinRT binding for Windows.Media.Ocr, no cgo.
// IIDs and vtable order verified against microsoft/windows-rs generated
// bindings and Wine's WinRT IDL headers.

import (
	"errors"
	"fmt"
	"image"
	"syscall"
	"time"
	"unsafe"
)

var (
	combase                       = syscall.NewLazyDLL("combase.dll")
	procRoInitialize              = combase.NewProc("RoInitialize")
	procRoUninitialize            = combase.NewProc("RoUninitialize")
	procRoGetActivationFactory    = combase.NewProc("RoGetActivationFactory")
	procWindowsCreateString       = combase.NewProc("WindowsCreateString")
	procWindowsDeleteString       = combase.NewProc("WindowsDeleteString")
	procWindowsGetStringRawBuffer = combase.NewProc("WindowsGetStringRawBuffer")
)

type guid struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

var (
	iidOcrEngineStatics           = guid{0x5bffa85a, 0x3384, 0x3540, [8]byte{0x99, 0x40, 0x69, 0x91, 0x20, 0xd4, 0x28, 0xa8}}
	iidSoftwareBitmapStatics      = guid{0xdf0385db, 0x672f, 0x4a9d, [8]byte{0x80, 0x6e, 0xc2, 0x44, 0x2f, 0x34, 0x3e, 0x86}}
	iidCryptographicBufferStatics = guid{0x320b7e22, 0x3cb0, 0x4cdf, [8]byte{0x86, 0x63, 0x1d, 0x28, 0x91, 0x00, 0x65, 0xeb}}
	iidLanguageFactory            = guid{0x9b0252ac, 0x0c27, 0x44f8, [8]byte{0xb7, 0x92, 0x97, 0x93, 0xfb, 0x66, 0xc6, 0x3e}}
	iidAsyncInfo                  = guid{0x00000036, 0x0000, 0x0000, [8]byte{0xc0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
)

// vtable slots (IUnknown 0-2, IInspectable 3-5)
const (
	slotQueryInterface = 0
	slotRelease        = 2

	slotOcrStaticsTryCreateFromLanguage    = 9
	slotOcrStaticsTryCreateFromUserProfile = 10
	slotOcrEngineRecognizeAsync            = 6
	slotOcrResultLines                     = 6
	slotOcrLineWords                       = 6
	slotOcrLineText                        = 7
	slotOcrWordBoundingRect                = 6
	slotOcrWordText                        = 7
	slotSoftwareBitmapCreateCopyFromBuffer = 9
	slotCryptoBufferCreateFromByteArray    = 9
	slotLanguageFactoryCreateLanguage      = 6
	slotAsyncInfoStatus                    = 7
	slotAsyncInfoErrorCode                 = 8
	slotAsyncOperationGetResults           = 8
	slotVectorViewGetAt                    = 6
	slotVectorViewSize                     = 7

	bitmapPixelFormatBgra8 = 87
)

type comObj uintptr

// call invokes a vtable method. uintptrescapes keeps any &out pointers passed
// as uintptr on the heap and alive for the duration of the call.
//
//go:uintptrescapes
//go:noinline
func (o comObj) call(slot int, args ...uintptr) uintptr {
	vtbl := *(*uintptr)(unsafe.Pointer(o))
	fn := *(*uintptr)(unsafe.Pointer(vtbl + uintptr(slot)*unsafe.Sizeof(uintptr(0))))
	r, _, _ := syscall.SyscallN(fn, append([]uintptr{uintptr(o)}, args...)...)
	return r
}

func (o comObj) release() {
	if o != 0 {
		o.call(slotRelease)
	}
}

func hr(r uintptr, what string) error {
	if int32(r) < 0 {
		return fmt.Errorf("%s: HRESULT 0x%08X", what, uint32(r))
	}
	return nil
}

func newHString(s string) (uintptr, error) {
	u, err := syscall.UTF16FromString(s)
	if err != nil {
		return 0, err
	}
	var h uintptr
	r, _, _ := procWindowsCreateString.Call(uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)-1), uintptr(unsafe.Pointer(&h)))
	return h, hr(r, "WindowsCreateString")
}

func deleteHString(h uintptr) {
	if h != 0 {
		procWindowsDeleteString.Call(h)
	}
}

func hstringToString(h uintptr) string {
	if h == 0 {
		return ""
	}
	var n uint32
	p, _, _ := procWindowsGetStringRawBuffer.Call(h, uintptr(unsafe.Pointer(&n)))
	if p == 0 || n == 0 {
		return ""
	}
	return syscall.UTF16ToString(unsafe.Slice((*uint16)(unsafe.Pointer(p)), n))
}

func activationFactory(class string, iid *guid) (comObj, error) {
	h, err := newHString(class)
	if err != nil {
		return 0, err
	}
	defer deleteHString(h)
	var f comObj
	r, _, _ := procRoGetActivationFactory.Call(h, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&f)))
	if err := hr(r, "RoGetActivationFactory "+class); err != nil {
		return 0, err
	}
	if f == 0 {
		return 0, errors.New("factory nil: " + class)
	}
	return f, nil
}

// OcrEngine wraps Windows.Media.Ocr.OcrEngine. Use from a single locked OS thread.
const (
	slotOcrStaticsAvailableLanguages = 7
	slotLanguageTag                  = 6
	slotLanguageDisplayName          = 7
)

type OcrEngine struct {
	engine  comObj
	bmpStat comObj
	bufStat comObj
	initRo  bool
	Lang    string    // tag requested ("" = Windows user languages)
	Langs   []OcrLang // recognizer languages installed on this PC
}

func languagesFrom(stat comObj) []OcrLang {
	var vec comObj
	if stat.call(slotOcrStaticsAvailableLanguages, uintptr(unsafe.Pointer(&vec))); vec == 0 {
		return nil
	}
	defer vec.release()
	var n uint32
	vec.call(slotVectorViewSize, uintptr(unsafe.Pointer(&n)))
	var out []OcrLang
	for i := uint32(0); i < n && i < 64; i++ {
		var lang comObj
		if vec.call(slotVectorViewGetAt, uintptr(i), uintptr(unsafe.Pointer(&lang))); lang == 0 {
			continue
		}
		var ht, hn uintptr
		lang.call(slotLanguageTag, uintptr(unsafe.Pointer(&ht)))
		lang.call(slotLanguageDisplayName, uintptr(unsafe.Pointer(&hn)))
		out = append(out, OcrLang{Tag: hstringToString(ht), Name: hstringToString(hn)})
		deleteHString(ht)
		deleteHString(hn)
		lang.release()
	}
	return out
}

// NewOcrEngine must be called on the thread that will use it (runtime.LockOSThread).
// NewOcrEngine creates an engine for tag (e.g. "ja-JP"); "" uses the Windows
// user languages. Falls back to English when the requested language is missing.
func NewOcrEngine(tag string) (*OcrEngine, error) {
	e := &OcrEngine{Lang: tag}
	r, _, _ := procRoInitialize.Call(1) // RO_INIT_MULTITHREADED
	// S_OK, S_FALSE (already) are fine; RPC_E_CHANGED_MODE means some other mode, still usable.
	if int32(r) >= 0 {
		e.initRo = true
	}
	ok := false
	defer func() {
		if !ok {
			e.Close()
		}
	}()

	stat, err := activationFactory("Windows.Media.Ocr.OcrEngine", &iidOcrEngineStatics)
	if err != nil {
		return nil, err
	}
	defer stat.release()
	e.Langs = languagesFrom(stat)

	var eng comObj
	if tag == "" {
		stat.call(slotOcrStaticsTryCreateFromUserProfile, uintptr(unsafe.Pointer(&eng)))
	}
	if eng == 0 {
		// User profile languages have no OCR support: fall back to English explicitly.
		lf, err := activationFactory("Windows.Globalization.Language", &iidLanguageFactory)
		if err != nil {
			return nil, err
		}
		defer lf.release()
		tags := []string{"en-US", "en-GB"}
		if tag != "" {
			tags = append([]string{tag}, tags...)
		}
		for _, tg := range tags {
			h, err := newHString(tg)
			if err != nil {
				continue
			}
			var lang comObj
			lf.call(slotLanguageFactoryCreateLanguage, h, uintptr(unsafe.Pointer(&lang)))
			deleteHString(h)
			if lang == 0 {
				continue
			}
			stat.call(slotOcrStaticsTryCreateFromLanguage, uintptr(lang), uintptr(unsafe.Pointer(&eng)))
			lang.release()
			if eng != 0 {
				break
			}
		}
	}
	if eng == 0 {
		return nil, errors.New("không tạo được OCR engine (máy chưa có gói ngôn ngữ hỗ trợ OCR)")
	}
	e.engine = eng

	if e.bmpStat, err = activationFactory("Windows.Graphics.Imaging.SoftwareBitmap", &iidSoftwareBitmapStatics); err != nil {
		return nil, err
	}
	if e.bufStat, err = activationFactory("Windows.Security.Cryptography.CryptographicBuffer", &iidCryptographicBufferStatics); err != nil {
		return nil, err
	}
	ok = true
	return e, nil
}

func (e *OcrEngine) Close() {
	if e == nil {
		return
	}
	e.engine.release()
	e.bmpStat.release()
	e.bufStat.release()
	e.engine, e.bmpStat, e.bufStat = 0, 0, 0
	if e.initRo {
		procRoUninitialize.Call()
		e.initRo = false
	}
}

// Recognize runs OCR on an RGBA image and returns lines with normalized word boxes.
func (e *OcrEngine) Recognize(img *image.RGBA) ([]OcrLine, error) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w == 0 || h == 0 {
		return nil, nil
	}
	// RGBA -> BGRA (opaque)
	buf := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		src := img.Pix[y*img.Stride : y*img.Stride+w*4]
		dst := buf[y*w*4 : (y+1)*w*4]
		for i := 0; i < w*4; i += 4 {
			dst[i], dst[i+1], dst[i+2], dst[i+3] = src[i+2], src[i+1], src[i], 255
		}
	}

	var ibuf comObj
	r := e.bufStat.call(slotCryptoBufferCreateFromByteArray, uintptr(len(buf)), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&ibuf)))
	if err := hr(r, "CreateFromByteArray"); err != nil {
		return nil, err
	}
	defer ibuf.release()

	var sbmp comObj
	r = e.bmpStat.call(slotSoftwareBitmapCreateCopyFromBuffer, uintptr(ibuf), bitmapPixelFormatBgra8, uintptr(w), uintptr(h), uintptr(unsafe.Pointer(&sbmp)))
	if err := hr(r, "SoftwareBitmap.CreateCopyFromBuffer"); err != nil {
		return nil, err
	}
	defer sbmp.release()

	var op comObj
	r = e.engine.call(slotOcrEngineRecognizeAsync, uintptr(sbmp), uintptr(unsafe.Pointer(&op)))
	if err := hr(r, "RecognizeAsync"); err != nil {
		return nil, err
	}
	defer op.release()

	var info comObj
	r = op.call(slotQueryInterface, uintptr(unsafe.Pointer(&iidAsyncInfo)), uintptr(unsafe.Pointer(&info)))
	if err := hr(r, "QueryInterface IAsyncInfo"); err != nil {
		return nil, err
	}
	defer info.release()

	deadline := time.Now().Add(5 * time.Second)
	for {
		var status int32
		info.call(slotAsyncInfoStatus, uintptr(unsafe.Pointer(&status)))
		if status == 1 { // Completed
			break
		}
		if status == 2 || status == 3 { // Canceled / Error
			var code int32
			info.call(slotAsyncInfoErrorCode, uintptr(unsafe.Pointer(&code)))
			return nil, fmt.Errorf("OCR lỗi (status %d, 0x%08X)", status, uint32(code))
		}
		if time.Now().After(deadline) {
			return nil, errors.New("OCR quá thời gian")
		}
		time.Sleep(5 * time.Millisecond)
	}

	var res comObj
	r = op.call(slotAsyncOperationGetResults, uintptr(unsafe.Pointer(&res)))
	if err := hr(r, "GetResults"); err != nil {
		return nil, err
	}
	if res == 0 {
		return nil, nil
	}
	defer res.release()

	var linesVec comObj
	if err := hr(res.call(slotOcrResultLines, uintptr(unsafe.Pointer(&linesVec))), "Lines"); err != nil || linesVec == 0 {
		return nil, err
	}
	defer linesVec.release()

	var nLines uint32
	linesVec.call(slotVectorViewSize, uintptr(unsafe.Pointer(&nLines)))
	fw, fh := float64(w), float64(h)
	var out []OcrLine
	for i := uint32(0); i < nLines && i < 200; i++ {
		var ln comObj
		if linesVec.call(slotVectorViewGetAt, uintptr(i), uintptr(unsafe.Pointer(&ln))); ln == 0 {
			continue
		}
		var ht uintptr
		ln.call(slotOcrLineText, uintptr(unsafe.Pointer(&ht)))
		line := OcrLine{Text: hstringToString(ht)}
		deleteHString(ht)

		var wordsVec comObj
		ln.call(slotOcrLineWords, uintptr(unsafe.Pointer(&wordsVec)))
		if wordsVec != 0 {
			var nWords uint32
			wordsVec.call(slotVectorViewSize, uintptr(unsafe.Pointer(&nWords)))
			for j := uint32(0); j < nWords && j < 100; j++ {
				var wd comObj
				if wordsVec.call(slotVectorViewGetAt, uintptr(j), uintptr(unsafe.Pointer(&wd))); wd == 0 {
					continue
				}
				var rect struct{ X, Y, W, H float32 }
				wd.call(slotOcrWordBoundingRect, uintptr(unsafe.Pointer(&rect)))
				var wt uintptr
				wd.call(slotOcrWordText, uintptr(unsafe.Pointer(&wt)))
				line.Words = append(line.Words, OcrWord{
					Text: hstringToString(wt),
					X:    float64(rect.X) / fw, Y: float64(rect.Y) / fh,
					W: float64(rect.W) / fw, H: float64(rect.H) / fh,
				})
				deleteHString(wt)
				wd.release()
			}
			wordsVec.release()
		}
		ln.release()
		out = append(out, line)
	}
	return out, nil
}
