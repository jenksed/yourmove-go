import AppKit
import CoreGraphics
import Foundation
let pid = Int32(CommandLine.arguments[1])!
let app = NSRunningApplication(processIdentifier: pid)
let windows = CGWindowListCopyWindowInfo([.optionAll, .excludeDesktopElements], kCGNullWindowID) as? [[String: Any]] ?? []
let owned = windows.filter { ($0[kCGWindowOwnerPID as String] as? Int32) == pid }.map { w in
 return ["window_id": w[kCGWindowNumber as String] ?? 0, "owner_pid": w[kCGWindowOwnerPID as String] ?? 0, "title": w[kCGWindowName as String] ?? "", "layer": w[kCGWindowLayer as String] ?? 0, "onscreen": w[kCGWindowIsOnscreen as String] ?? false] as [String: Any]
}
let result: [String: Any] = ["pid": pid, "executable": app?.executableURL?.path ?? "", "bundle": app?.bundleURL?.path ?? "", "launch_date": app?.launchDate?.description ?? "", "windows": owned]
let data = try JSONSerialization.data(withJSONObject: result, options: [.prettyPrinted, .sortedKeys])
print(String(data: data, encoding: .utf8)!)
