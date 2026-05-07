// genie tray icon implementation. Compiled separately from the cgo preamble
// so the GenieTray Objective-C class is only emitted once per binary.
#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>

// Wails' AppDelegate calls -[NSApplication setActivationPolicy:Regular] in
// applicationWillFinishLaunching and then activateIgnoringOtherApps:YES in
// applicationDidFinishLaunching, which sticks a Dock icon for our menu-bar-
// only app. Switching back to Accessory afterwards is racy. We swizzle the
// setter at load time so every Regular request is silently rewritten to
// Accessory — the Dock entry is never registered to begin with.
__attribute__((constructor))
static void genie_force_accessory_policy(void) {
    static Method method = NULL;
    static IMP   originalImp = NULL;
    method = class_getInstanceMethod([NSApplication class], @selector(setActivationPolicy:));
    if (method == NULL) return;
    originalImp = method_getImplementation(method);

    IMP override = imp_implementationWithBlock(^(NSApplication *self, NSApplicationActivationPolicy requested) {
        NSApplicationActivationPolicy p = (requested == NSApplicationActivationPolicyRegular)
            ? NSApplicationActivationPolicyAccessory
            : requested;
        ((void(*)(id, SEL, NSApplicationActivationPolicy))originalImp)(self, @selector(setActivationPolicy:), p);
    });
    method_setImplementation(method, override);
}

extern void genieTrayToggle(void);
extern void genieTraySettings(void);
extern void genieTrayQuit(void);

@interface GenieTray : NSObject {
    NSStatusItem* item;
    NSMenu* menu;
}
- (void)create;
- (void)setRecording:(BOOL)on;
- (void)onClick:(id)sender;
- (void)onMenuToggle:(id)sender;
- (void)onSettings:(id)sender;
- (void)onQuit:(id)sender;
@end

@implementation GenieTray

- (void)create {
    item = [[NSStatusBar systemStatusBar] statusItemWithLength:NSVariableStatusItemLength];
    [self setRecording:NO];

    // Build the menu but do NOT attach it to the status item — that would make
    // every click open it. We pop it manually for right/ctrl clicks below.
    menu = [[NSMenu alloc] init];
    NSMenuItem* toggle = [[NSMenuItem alloc] initWithTitle:@"Toggle Recording"
                                                     action:@selector(onMenuToggle:)
                                              keyEquivalent:@""];
    [toggle setTarget:self];
    [menu addItem:toggle];
    [menu addItem:[NSMenuItem separatorItem]];
    NSMenuItem* settings = [[NSMenuItem alloc] initWithTitle:@"Settings…"
                                                       action:@selector(onSettings:)
                                                keyEquivalent:@","];
    [settings setTarget:self];
    [menu addItem:settings];
    [menu addItem:[NSMenuItem separatorItem]];
    NSMenuItem* quit = [[NSMenuItem alloc] initWithTitle:@"Quit Genie"
                                                   action:@selector(onQuit:)
                                            keyEquivalent:@"q"];
    [quit setTarget:self];
    [menu addItem:quit];

    NSStatusBarButton* button = [item button];
    [button setTarget:self];
    [button setAction:@selector(onClick:)];
    // Receive both primary and secondary clicks so we can distinguish them.
    [button sendActionOn:(NSEventMaskLeftMouseUp | NSEventMaskRightMouseUp)];
}

- (void)setRecording:(BOOL)on {
    NSString* sym = on ? @"record.circle.fill" : @"mic";
    NSString* desc = on ? @"Genie recording" : @"Genie idle";
    if (@available(macOS 11.0, *)) {
        NSImage* img = [NSImage imageWithSystemSymbolName:sym accessibilityDescription:desc];
        [img setTemplate:YES];
        [[item button] setImage:img];
    } else {
        [[item button] setTitle:on ? @"●" : @"◯"];
    }
}

- (void)onClick:(id)sender {
    NSEvent* event = [NSApp currentEvent];
    BOOL secondary = NO;
    if (event != nil) {
        if (event.type == NSEventTypeRightMouseUp || event.type == NSEventTypeRightMouseDown) {
            secondary = YES;
        } else if ((event.modifierFlags & NSEventModifierFlagControl) != 0) {
            // Ctrl+click is the macOS convention for secondary click.
            secondary = YES;
        }
    }

    if (secondary) {
        // Pop the menu under the status item button.
        NSStatusBarButton* button = [item button];
        [NSMenu popUpContextMenu:menu withEvent:event forView:button];
        return;
    }

    genieTrayToggle();
}

- (void)onMenuToggle:(id)sender { (void)sender; genieTrayToggle(); }
- (void)onSettings:(id)sender { (void)sender; genieTraySettings(); }
- (void)onQuit:(id)sender { (void)sender; genieTrayQuit(); }
@end

static GenieTray* gTray = nil;

void genie_tray_create(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (gTray == nil) {
            gTray = [[GenieTray alloc] init];
            [gTray create];
        }
    });
}

void genie_tray_set_recording(int on) {
    BOOL b = on ? YES : NO;
    dispatch_async(dispatch_get_main_queue(), ^{
        if (gTray != nil) [gTray setRecording:b];
    });
}
