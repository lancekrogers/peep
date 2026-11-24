// Run by the system JavaScript for Automation interpreter. No compiler or
// separately installed app is needed. The file path arrives only in argv.
ObjC.import('AppKit');

function run(argv) {
    var path = $(argv[0]);
    var url = $.NSURL.fileURLWithPath(path);
    var app = $.NSApplication.sharedApplication;
    app.setActivationPolicy($.NSApplicationActivationPolicyAccessory);
    var icon = $.NSWorkspace.sharedWorkspace.iconForFile(path);
    icon.size = $.NSMakeSize(96, 96);

    ObjC.registerSubclass({
        name: 'PeepDragView',
        superclass: 'NSImageView',
        protocols: ['NSDraggingSource'],
        methods: {
            'acceptsFirstMouse:': {
                types: ['bool', ['id']],
                implementation: function() { return true; }
            },
            'mouseDown:': {
                types: ['void', ['id']],
                implementation: function(event) {
                    var item = $.NSDraggingItem.alloc.initWithPasteboardWriter(url);
                    item.setDraggingFrameContents($.NSMakeRect(32, 12, 96, 96), icon);
                    // JXA bridges a JS array of Objective-C wrappers into
                    // dictionaries. AppKit needs actual NSDraggingItem objects.
                    var items = $.NSMutableArray.alloc.init;
                    items.addObject(item);
                    this.beginDraggingSessionWithItemsEventSource(items, event, this);
                }
            },
            'draggingSession:sourceOperationMaskForDraggingContext:': {
                implementation: function() { return $.NSDragOperationCopy; }
            },
            'draggingSession:endedAtPoint:operation:': {
                implementation: function(session, point, operation) {
                    if (operation !== 0) app.terminate(null);
                }
            }
        }
    });
    ObjC.registerSubclass({
        name: 'PeepDragDelegate',
        superclass: 'NSObject',
        protocols: ['NSWindowDelegate'],
        methods: {
            'windowWillClose:': {
                types: ['void', ['id']],
                implementation: function() { app.terminate(null); }
            }
        }
    });
    var window = $.NSWindow.alloc.initWithContentRectStyleMaskBackingDefer(
        $.NSMakeRect(0, 0, 360, 190),
        $.NSWindowStyleMaskTitled | $.NSWindowStyleMaskClosable,
        $.NSBackingStoreBuffered, false);
    window.title = 'peep — drag into another app';
    window.releasedWhenClosed = false;
    var delegate = $.PeepDragDelegate.alloc.init;
    window.delegate = delegate;
    var view = $.PeepDragView.alloc.initWithFrame($.NSMakeRect(100, 44, 160, 116));
    view.image = icon;
    view.imageScaling = $.NSImageScaleProportionallyUpOrDown;
    window.contentView.addSubview(view);
    var label = $.NSTextField.labelWithString(path.lastPathComponent);
    label.frame = $.NSMakeRect(16, 16, 328, 24);
    label.alignment = $.NSTextAlignmentCenter;
    label.lineBreakMode = $.NSLineBreakByTruncatingMiddle;
    window.contentView.addSubview(label);
    var hint = $.NSTextField.labelWithString('Drag the file icon into another app');
    hint.frame = $.NSMakeRect(16, 166, 328, 20);
    hint.alignment = $.NSTextAlignmentCenter;
    hint.textColor = $.NSColor.secondaryLabelColor;
    window.contentView.addSubview(hint);
    window.center;
    window.makeKeyAndOrderFront(null);
    app.activateIgnoringOtherApps(true);
    app.run;
}
