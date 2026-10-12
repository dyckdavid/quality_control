//
//  QualityControlUITests.swift
//  QualityControlUITests
//
//  Created by David Dyck on 29/09/26.
//

import XCTest

final class QualityControlUITests: XCTestCase {

    override func setUpWithError() throws {
        // Put setup code here. This method is called before the invocation of each test method in the class.

        // In UI tests it is usually best to stop immediately when a failure occurs.
        continueAfterFailure = false

        // In UI tests it’s important to set the initial state - such as interface orientation - required for your tests before they run. The setUp method is a good place to do this.
    }

    override func tearDownWithError() throws {
        // Put teardown code here. This method is called after the invocation of each test method in the class.
    }

    @MainActor
    func testExample() throws {
        // UI tests must launch the application that they test.
        let app = XCUIApplication()
        app.launch()

        // Use XCTAssert and related functions to verify your tests produce the correct results.
        // XCUIAutomation Documentation
        // https://developer.apple.com/documentation/xcuiautomation
    }

    /// Signs in and saves a screenshot of each main screen. Set QC_SHOT_SERVER, QC_SHOT_USER, QC_SHOT_PASSWORD,
    /// and QC_SHOT_DIR (pass them to xcodebuild with a TEST_RUNNER_ prefix) to run it.
    @MainActor
    func testScreenshotTour() throws {
        let env = ProcessInfo.processInfo.environment
        guard let server = env["QC_SHOT_SERVER"], let user = env["QC_SHOT_USER"], let password = env["QC_SHOT_PASSWORD"] else {
            throw XCTSkip("Screenshot tour needs QC_SHOT_SERVER, QC_SHOT_USER, and QC_SHOT_PASSWORD")
        }
        let dir = URL(fileURLWithPath: env["QC_SHOT_DIR"] ?? NSTemporaryDirectory())
        try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        let app = XCUIApplication()
        app.launch()

        func shot(_ name: String) {
            sleep(2)
            let image = XCUIScreen.main.screenshot()
            try? image.pngRepresentation.write(to: dir.appendingPathComponent("\(name).png"))
            let attachment = XCTAttachment(screenshot: image)
            attachment.name = name
            attachment.lifetime = .keepAlways
            add(attachment)
        }

        let signIn = app.buttons["Sign in"]
        if signIn.waitForExistence(timeout: 5) {
            let serverField = app.textFields["Server URL"]
            serverField.tap()
            serverField.press(forDuration: 1.2)
            if app.menuItems["Select All"].waitForExistence(timeout: 2) { app.menuItems["Select All"].tap() }
            serverField.typeText(server)
            let userField = app.textFields["Username"]
            userField.tap()
            if let current = userField.value as? String, !current.isEmpty, current != "Username" {
                userField.typeText(String(repeating: XCUIKeyboardKey.delete.rawValue, count: current.count))
            }
            userField.typeText(user)
            app.secureTextFields["Password"].tap()
            app.secureTextFields["Password"].typeText(password)
            signIn.tap()
        }
        XCTAssertTrue(app.staticTexts["Scan a tag"].waitForExistence(timeout: 15))
        shot("01_home")

        app.buttons["Scan"].firstMatch.tap()
        shot("02_scan")
        let codeField = app.textFields["Code"]
        if codeField.waitForExistence(timeout: 5) {
            codeField.tap()
            codeField.typeText("LOT-1\n")
            shot("03_scan_result")
        }

        app.buttons["Lots"].firstMatch.tap()
        shot("04_lots")
        app.buttons["Alerts"].firstMatch.tap()
        shot("04b_alerts")

        if app.buttons["More"].exists {
            app.buttons["More"].firstMatch.tap()
            shot("05_more")
        } else if app.buttons["Shop setup"].exists {
            app.buttons["Shop setup"].firstMatch.tap()
            shot("05_shop_setup")
        }
        app.staticTexts["Products"].firstMatch.tap()
        shot("06_products")
        let firstProduct = app.staticTexts["BRK-100"].firstMatch
        XCTAssertTrue(firstProduct.waitForExistence(timeout: 5))
        firstProduct.tap()
        shot("07_product_editor")
        app.swipeUp()
        shot("08_product_label")
    }

    @MainActor
    func testLaunchPerformance() throws {
        // This measures how long it takes to launch your application.
        measure(metrics: [XCTApplicationLaunchMetric()]) {
            XCUIApplication().launch()
        }
    }
}
