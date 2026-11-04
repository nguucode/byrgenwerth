#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#import <ScreenCaptureKit/ScreenCaptureKit.h>
#import <Vision/Vision.h>
#import <Carbon/Carbon.h>
#import <Security/Security.h>
#include "platform_darwin.h"

extern void goHotkey(int id);
extern void goWillQuit(void);

static void onMain(dispatch_block_t b) {
	if ([NSThread isMainThread]) b();
	else dispatch_async(dispatch_get_main_queue(), b);
}

static void onMainWait(dispatch_block_t b) {
	if ([NSThread isMainThread]) b();
	else dispatch_sync(dispatch_get_main_queue(), b);
}

static char *dupNS(NSString *s) { return strdup(s ? s.UTF8String : ""); }

static char *dupJSON(id obj) {
	NSData *d = [NSJSONSerialization dataWithJSONObject:obj options:0 error:nil];
	return d ? dupNS([[NSString alloc] initWithData:d encoding:NSUTF8StringEncoding]) : strdup("[]");
}

// ---------- app ----------

static NSWindow *mainWindow;
static NSString *mainURL;
static NSMutableArray<NSPanel *> *overlays;
static NSStatusItem *statusItem;

static WKWebView *newWebView(NSRect frame, NSString *url) {
	WKWebView *v = [[WKWebView alloc] initWithFrame:frame configuration:[WKWebViewConfiguration new]];
	v.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
	[v loadRequest:[NSURLRequest requestWithURL:[NSURL URLWithString:url]]];
	return v;
}

@interface BWDelegate : NSObject <NSApplicationDelegate, NSWindowDelegate>
@end

@implementation BWDelegate
- (void)applicationDidFinishLaunching:(NSNotification *)n {
	NSRect r = mainWindow.frame;
	[mainWindow setContentView:newWebView(NSMakeRect(0, 0, r.size.width, r.size.height), mainURL)];
	[mainWindow center];
	[NSApp activateIgnoringOtherApps:YES];
	[mainWindow makeKeyAndOrderFront:nil];
}
- (BOOL)windowShouldClose:(NSWindow *)w {
	[NSApp terminate:nil];
	return NO;
}
- (void)applicationWillTerminate:(NSNotification *)n { goWillQuit(); }
- (void)showMain:(id)sender { bw_main_show(); }
@end

static BWDelegate *delegate;

static NSMenuItem *menuOf(NSString *title, NSArray<NSMenuItem *> *items) {
	NSMenuItem *top = [NSMenuItem new];
	NSMenu *m = [[NSMenu alloc] initWithTitle:title];
	for (NSMenuItem *i in items) [m addItem:i];
	top.submenu = m;
	return top;
}

static NSMenuItem *item(NSString *title, SEL action, NSString *key) {
	return [[NSMenuItem alloc] initWithTitle:title action:action keyEquivalent:key];
}

void bw_run(const char *url, int width, int height) {
	@autoreleasepool {
		[NSApplication sharedApplication];
		// An accessory app: its panels can sit above another app's full-screen space.
		[NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
		mainURL = @(url);
		overlays = [NSMutableArray new];
		delegate = [BWDelegate new];
		NSApp.delegate = delegate;

		// Without these menu items, copy and paste do not reach the web view.
		NSMenu *bar = [NSMenu new];
		[bar addItem:menuOf(@"Byrgenwerth", @[item(@"Quit Byrgenwerth", @selector(terminate:), @"q")])];
		[bar addItem:menuOf(@"Edit", @[
			item(@"Undo", NSSelectorFromString(@"undo:"), @"z"),
			item(@"Cut", @selector(cut:), @"x"),
			item(@"Copy", @selector(copy:), @"c"),
			item(@"Paste", @selector(paste:), @"v"),
			item(@"Select All", @selector(selectAll:), @"a"),
		])];
		NSApp.mainMenu = bar;

		// No Dock icon in an accessory app, so the menu bar is the way back.
		statusItem = [NSStatusBar.systemStatusBar statusItemWithLength:NSVariableStatusItemLength];
		statusItem.button.title = @"Byrgenwerth";
		NSMenu *sm = [NSMenu new];
		NSMenuItem *show = item(@"Mở cửa sổ", @selector(showMain:), @"");
		show.target = delegate;
		[sm addItem:show];
		[sm addItem:item(@"Thoát", @selector(terminate:), @"")];
		statusItem.menu = sm;

		mainWindow = [[NSWindow alloc] initWithContentRect:NSMakeRect(0, 0, width, height)
			styleMask:NSWindowStyleMaskTitled | NSWindowStyleMaskClosable | NSWindowStyleMaskMiniaturizable | NSWindowStyleMaskResizable
			backing:NSBackingStoreBuffered defer:NO];
		mainWindow.title = @"Byrgenwerth";
		mainWindow.delegate = delegate;
		mainWindow.releasedWhenClosed = NO;
		// F7 or the region hotkey inside a full-screen game opens here, not on another Space.
		mainWindow.collectionBehavior = NSWindowCollectionBehaviorMoveToActiveSpace | NSWindowCollectionBehaviorFullScreenAuxiliary;
		[NSApp run];
	}
}

void bw_quit(void) { onMain(^{ [NSApp terminate:nil]; }); }

void bw_main_show(void) {
	onMain(^{
		[NSApp activateIgnoringOtherApps:YES];
		[mainWindow deminiaturize:nil];
		[mainWindow makeKeyAndOrderFront:nil];
	});
}

void bw_main_hide(void) { onMain(^{ [mainWindow orderOut:nil]; }); }

int bw_main_visible(void) {
	__block int v = 0;
	onMainWait(^{ v = mainWindow.isVisible && !mainWindow.isMiniaturized; });
	return v;
}

void bw_main_resize(int width, int height) {
	onMain(^{
		NSRect r = mainWindow.frame;
		NSRect want = [mainWindow frameRectForContentRect:NSMakeRect(0, 0, width, height)];
		r.origin.y += r.size.height - want.size.height; // keep the top edge in place
		r.size = want.size;
		NSRect vis = (mainWindow.screen ?: NSScreen.mainScreen).visibleFrame;
		r.origin.x = MAX(vis.origin.x, MIN(r.origin.x, NSMaxX(vis) - r.size.width));
		r.origin.y = MAX(vis.origin.y, MIN(r.origin.y, NSMaxY(vis) - r.size.height));
		[mainWindow setFrame:r display:YES animate:NO];
	});
}

void bw_main_top(int on) {
	onMain(^{ mainWindow.level = on ? NSFloatingWindowLevel : NSNormalWindowLevel; });
}

// ---------- overlays ----------

// DragCover sits over an overlay's web view in edit mode: it shows the yellow
// frame and lets the whole window be dragged.
@interface DragCover : NSView
@end

@implementation DragCover
- (BOOL)mouseDownCanMoveWindow { return YES; }
- (void)drawRect:(NSRect)r {
	[[NSColor colorWithWhite:0.1 alpha:0.45] setFill];
	NSRectFill(self.bounds);
	[[NSColor colorWithRed:1 green:0.78 blue:0.16 alpha:0.95] setStroke];
	NSBezierPath *p = [NSBezierPath bezierPathWithRect:NSInsetRect(self.bounds, 1, 1)];
	p.lineWidth = 2;
	[p stroke];
}
@end

static CGFloat primaryHeight(void) { return NSScreen.screens.firstObject.frame.size.height; }

int bw_overlay_create(const char *curl) {
	__block int id = -1;
	NSString *url = @(curl);
	onMainWait(^{
		NSPanel *p = [[NSPanel alloc] initWithContentRect:NSMakeRect(0, 0, 400, 100)
			styleMask:NSWindowStyleMaskBorderless | NSWindowStyleMaskNonactivatingPanel
			backing:NSBackingStoreBuffered defer:NO];
		p.opaque = NO;
		p.backgroundColor = NSColor.clearColor;
		p.hasShadow = NO;
		p.ignoresMouseEvents = YES;
		p.level = NSScreenSaverWindowLevel;
		p.collectionBehavior = NSWindowCollectionBehaviorCanJoinAllSpaces | NSWindowCollectionBehaviorFullScreenAuxiliary | NSWindowCollectionBehaviorStationary;
		p.hidesOnDeactivate = NO;
		p.movableByWindowBackground = YES;
		WKWebView *v = newWebView(NSMakeRect(0, 0, 400, 100), url);
		[v setValue:@NO forKey:@"drawsBackground"];
		p.contentView = v;
		[overlays addObject:p];
		id = (int)overlays.count - 1;
	});
	return id;
}

void bw_overlay_frame(int id, int x, int y, int w, int h) {
	onMain(^{
		NSRect r = NSMakeRect(x, primaryHeight() - y - h, w, h);
		[overlays[id] setFrame:r display:YES];
	});
}

void bw_overlay_show(int id, int show) {
	onMain(^{
		if (show) [overlays[id] orderFrontRegardless];
		else [overlays[id] orderOut:nil];
	});
}

void bw_overlay_edit(int id, int on) {
	onMain(^{
		NSPanel *p = overlays[id];
		p.ignoresMouseEvents = !on;
		for (NSView *s in [p.contentView.subviews copy]) {
			if ([s isKindOfClass:DragCover.class]) [s removeFromSuperview];
		}
		if (on) {
			DragCover *c = [[DragCover alloc] initWithFrame:p.contentView.bounds];
			c.autoresizingMask = NSViewWidthSizable | NSViewHeightSizable;
			[p.contentView addSubview:c];
		}
	});
}

void bw_overlay_origin(int id, int *x, int *y) {
	__block NSRect r;
	onMainWait(^{ r = overlays[id].frame; });
	*x = (int)r.origin.x;
	*y = (int)(primaryHeight() - r.origin.y - r.size.height);
}

// ---------- screens ----------

int bw_screens(int32_t *out, int max) {
	__block int n = 0;
	onMainWait(^{
		CGFloat top = primaryHeight();
		for (NSScreen *s in NSScreen.screens) {
			if (n >= max) break;
			NSRect f = s.frame;
			int32_t *e = out + n * 5;
			e[0] = f.origin.x;
			e[1] = top - NSMaxY(f);
			e[2] = NSMaxX(f);
			e[3] = top - f.origin.y;
			e[4] = n == 0;
			n++;
		}
	});
	return n;
}

// The front app is another app and its top window fills a screen. loose also
// accepts a game in a large window, since many Mac games keep the menu bar in
// view. Uses CoreGraphics, so it works while a modal panel holds the main thread.
int bw_front_fullscreen(int loose) {
	@autoreleasepool {
		NSRunningApplication *front = NSWorkspace.sharedWorkspace.frontmostApplication;
		if (!front || front.processIdentifier == getpid()) return 0;
		int pid = front.processIdentifier;
		int result = 0;
		CFArrayRef list = CGWindowListCopyWindowInfo(kCGWindowListOptionOnScreenOnly | kCGWindowListExcludeDesktopElements, kCGNullWindowID);
		if (!list) return 0;
		CGDirectDisplayID displays[8];
		uint32_t nd = 0;
		CGGetActiveDisplayList(8, displays, &nd);
		double minW = loose ? 0.9 : 0.99, minH = loose ? 0.85 : 0.95; // a notch or menu bar takes a few percent
		for (NSDictionary *w in (__bridge NSArray *)list) {
			if ([w[(id)kCGWindowOwnerPID] intValue] != pid || [w[(id)kCGWindowLayer] intValue] != 0) continue;
			CGRect b;
			if (!CGRectMakeWithDictionaryRepresentation((__bridge CFDictionaryRef)w[(id)kCGWindowBounds], &b)) continue;
			for (uint32_t i = 0; i < nd; i++) {
				CGRect d = CGDisplayBounds(displays[i]);
				CGRect o = CGRectIntersection(b, d);
				if (o.size.width >= d.size.width * minW && o.size.height >= d.size.height * minH) result = 1;
			}
			break;
		}
		CFRelease(list);
		return result;
	}
}

// ---------- capture ----------

int bw_screen_access(int request) {
	if (CGPreflightScreenCaptureAccess()) return 1;
	return request ? CGRequestScreenCaptureAccess() : 0;
}

// Asking the system for its displays and apps is slow, so the answer is reused.
static SCShareableContent *shareable(NSError **err) {
	static SCShareableContent *cached;
	static CFAbsoluteTime at;
	static NSLock *lock;
	static dispatch_once_t once;
	dispatch_once(&once, ^{ lock = [NSLock new]; });
	[lock lock];
	SCShareableContent *c = cached;
	if (!c || CFAbsoluteTimeGetCurrent() - at > 5) {
		dispatch_semaphore_t done = dispatch_semaphore_create(0);
		__block SCShareableContent *got;
		__block NSError *e;
		[SCShareableContent getShareableContentExcludingDesktopWindows:NO onScreenWindowsOnly:YES completionHandler:^(SCShareableContent *sc, NSError *er) {
			got = sc;
			e = er;
			dispatch_semaphore_signal(done);
		}];
		dispatch_semaphore_wait(done, DISPATCH_TIME_FOREVER);
		if (got) {
			cached = c = got;
			at = CFAbsoluteTimeGetCurrent();
		} else if (err) {
			*err = e;
		}
	}
	[lock unlock];
	return c;
}

// bw_capture fills rgba (cw*ch*4 bytes) with a screen rectangle, leaving this
// app's own windows out, so OCR never reads the subtitle or the counter. Returns
// an error message, or NULL.
char *bw_capture(int x, int y, int w, int h, int cw, int ch, uint8_t *rgba) {
	@autoreleasepool {
		NSError *err;
		SCShareableContent *content = shareable(&err);
		if (!content) return dupNS(err.localizedDescription ?: @"không đọc được danh sách màn hình");
		CGPoint center = CGPointMake(x + w / 2.0, y + h / 2.0);
		SCDisplay *display;
		for (SCDisplay *d in content.displays) {
			if (CGRectContainsPoint(d.frame, center)) display = d;
		}
		if (!display) return strdup("vùng không nằm trên màn hình nào");
		NSMutableArray *mine = [NSMutableArray new];
		for (SCRunningApplication *a in content.applications) {
			if (a.processID == getpid()) [mine addObject:a];
		}
		SCContentFilter *filter = [[SCContentFilter alloc] initWithDisplay:display excludingApplications:mine exceptingWindows:@[]];
		SCStreamConfiguration *cfg = [SCStreamConfiguration new];
		cfg.width = cw;
		cfg.height = ch;
		cfg.sourceRect = CGRectMake(x - display.frame.origin.x, y - display.frame.origin.y, w, h);
		cfg.showsCursor = NO;

		dispatch_semaphore_t done = dispatch_semaphore_create(0);
		__block CGImageRef img = NULL;
		__block NSError *shotErr;
		[SCScreenshotManager captureImageWithFilter:filter configuration:cfg completionHandler:^(CGImageRef image, NSError *e) {
			if (image) img = CGImageRetain(image);
			shotErr = e;
			dispatch_semaphore_signal(done);
		}];
		dispatch_semaphore_wait(done, DISPATCH_TIME_FOREVER);
		if (!img) return dupNS(shotErr.localizedDescription ?: @"không chụp được màn hình");

		CGColorSpaceRef cs = CGColorSpaceCreateDeviceRGB();
		CGContextRef ctx = CGBitmapContextCreate(rgba, cw, ch, 8, cw * 4, cs, kCGImageAlphaNoneSkipLast | kCGBitmapByteOrder32Big);
		CGContextDrawImage(ctx, CGRectMake(0, 0, cw, ch), img);
		CGContextRelease(ctx);
		CGColorSpaceRelease(cs);
		CGImageRelease(img);
		for (int i = 3; i < cw * ch * 4; i += 4) rgba[i] = 255;
		return NULL;
	}
}

// ---------- OCR ----------

static NSNumber *num(double v) { return @(isfinite(v) ? v : 0); }

// bw_ocr returns JSON: {"lines":[{"text":..,"words":[{"t":..,"x":..,"y":..,"w":..,"h":..}]}]}
// with boxes normalized to a top-left origin, or {"error":..}.
char *bw_ocr(const uint8_t *rgba, int w, int h, const char *lang) {
	@autoreleasepool {
		CGColorSpaceRef cs = CGColorSpaceCreateDeviceRGB();
		CGContextRef ctx = CGBitmapContextCreate((void *)rgba, w, h, 8, w * 4, cs, kCGImageAlphaNoneSkipLast | kCGBitmapByteOrder32Big);
		CGImageRef img = CGBitmapContextCreateImage(ctx);
		CGContextRelease(ctx);
		CGColorSpaceRelease(cs);

		VNRecognizeTextRequest *req = [VNRecognizeTextRequest new];
		req.recognitionLevel = VNRequestTextRecognitionLevelAccurate;
		req.usesLanguageCorrection = YES;
		if (lang[0]) req.recognitionLanguages = @[@(lang)];
		else req.automaticallyDetectsLanguage = YES;
		NSError *err;
		VNImageRequestHandler *handler = [[VNImageRequestHandler alloc] initWithCGImage:img options:@{}];
		BOOL ok = [handler performRequests:@[req] error:&err];
		CGImageRelease(img);
		if (!ok) return dupJSON(@{@"error": err.localizedDescription ?: @"OCR lỗi"});

		// Vision's origin is bottom-left, so a larger minY is higher on screen.
		NSArray *obs = [req.results sortedArrayUsingComparator:^NSComparisonResult(VNRecognizedTextObservation *a, VNRecognizedTextObservation *b) {
			return a.boundingBox.origin.y > b.boundingBox.origin.y ? NSOrderedAscending : NSOrderedDescending;
		}];
		NSMutableArray *lines = [NSMutableArray new];
		for (VNRecognizedTextObservation *o in obs) {
			VNRecognizedText *t = [o topCandidates:1].firstObject;
			if (!t) continue;
			NSMutableArray *words = [NSMutableArray new];
			[t.string enumerateSubstringsInRange:NSMakeRange(0, t.string.length) options:NSStringEnumerationByWords usingBlock:^(NSString *word, NSRange r, NSRange er, BOOL *stop) {
				CGRect b = o.boundingBox;
				VNRectangleObservation *ro = [t boundingBoxForRange:r error:nil];
				if (ro) b = ro.boundingBox;
				[words addObject:@{@"t": word, @"x": num(b.origin.x), @"y": num(1 - CGRectGetMaxY(b)), @"w": num(b.size.width), @"h": num(b.size.height)}];
			}];
			[lines addObject:@{@"text": t.string, @"words": words}];
		}
		return dupJSON(@{@"lines": lines});
	}
}

char *bw_ocr_langs(void) {
	@autoreleasepool {
		VNRecognizeTextRequest *req = [VNRecognizeTextRequest new];
		req.recognitionLevel = VNRequestTextRecognitionLevelAccurate;
		NSArray *tags = [req supportedRecognitionLanguagesAndReturnError:nil] ?: @[];
		NSLocale *vi = [NSLocale localeWithLocaleIdentifier:@"vi"];
		NSMutableArray *out = [NSMutableArray new];
		for (NSString *tag in tags) {
			[out addObject:@{@"tag": tag, @"name": [vi displayNameForKey:NSLocaleIdentifier value:tag] ?: tag}];
		}
		return dupJSON(out);
	}
}

// ---------- hotkeys ----------

static OSStatus onHotkey(EventHandlerCallRef next, EventRef ev, void *data) {
	EventHotKeyID hk;
	GetEventParameter(ev, kEventParamDirectObject, typeEventHotKeyID, NULL, sizeof(hk), NULL, &hk);
	goHotkey((int)hk.id);
	return noErr;
}

int bw_hotkey(int id, int keycode, int mods) {
	__block OSStatus st = noErr;
	onMainWait(^{
		static dispatch_once_t once;
		dispatch_once(&once, ^{
			EventTypeSpec spec = {kEventClassKeyboard, kEventHotKeyPressed};
			InstallApplicationEventHandler(&onHotkey, 1, &spec, NULL, NULL);
		});
		EventHotKeyRef ref;
		st = RegisterEventHotKey(keycode, mods, (EventHotKeyID){'BYRG', (UInt32)id}, GetApplicationEventTarget(), 0, &ref);
	});
	return st == noErr;
}

// ---------- keychain ----------

static NSDictionary *keyQuery(void) {
	return @{
		(id)kSecClass: (id)kSecClassGenericPassword,
		(id)kSecAttrService: @"Byrgenwerth",
		(id)kSecAttrAccount: @"anthropic-api-key",
	};
}

// bw_key_save stores the API key; an empty key deletes it.
int bw_key_save(const char *key) {
	@autoreleasepool {
		SecItemDelete((__bridge CFDictionaryRef)keyQuery());
		if (!key[0]) return 1;
		NSMutableDictionary *q = [keyQuery() mutableCopy];
		q[(id)kSecValueData] = [@(key) dataUsingEncoding:NSUTF8StringEncoding];
		return SecItemAdd((__bridge CFDictionaryRef)q, NULL) == errSecSuccess;
	}
}

char *bw_key_load(void) {
	@autoreleasepool {
		NSMutableDictionary *q = [keyQuery() mutableCopy];
		q[(id)kSecReturnData] = @YES;
		CFTypeRef out = NULL;
		if (SecItemCopyMatching((__bridge CFDictionaryRef)q, &out) != errSecSuccess || !out) return strdup("");
		NSData *d = (__bridge_transfer NSData *)out;
		return dupNS([[NSString alloc] initWithData:d encoding:NSUTF8StringEncoding]);
	}
}

// ---------- helpers ----------

char *bw_font_families(void) {
	@autoreleasepool {
		return dupJSON(NSFontManager.sharedFontManager.availableFontFamilies ?: @[]);
	}
}

void bw_copy(const char *text) {
	NSString *s = @(text);
	onMain(^{
		[NSPasteboard.generalPasteboard clearContents];
		[NSPasteboard.generalPasteboard setString:s forType:NSPasteboardTypeString];
	});
}

char *bw_choose_file(void) {
	__block char *out = strdup("");
	onMainWait(^{
		NSOpenPanel *p = [NSOpenPanel openPanel];
		p.allowedFileTypes = @[@"mp3", @"wav", @"m4a", @"aiff"];
		if ([p runModal] == NSModalResponseOK) {
			free(out);
			out = dupNS(p.URL.path);
		}
	});
	return out;
}
