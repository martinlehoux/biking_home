package importer

import "github.com/playwright-community/playwright-go"
import "github.com/martinlehoux/kagamigo/kcore"
import "fmt"
import "net/http"
import "bytes"
import "io"

type GarminPlaywrightImporter struct {
}

func (i *GarminPlaywrightImporter) Run() {
	kcore.Expect(playwright.Install(), "failed to install browser")
	pw, err := playwright.Run()
	kcore.Expect(err, "failed to run playwright")
	browser, err := pw.Chromium.Launch()
	kcore.Expect(err, "failed to launch chromium")
	page, err := browser.NewPage()
	kcore.Expect(err, "failed to create page")
	_, err = page.Goto("https://connect.garmin.com/modern/home")
	kcore.Expect(err, "failed to navigate to home")
	emailInput := page.Locator("input#email")
	kcore.Expect(emailInput.Click(), "failed to click email input")
	var email string
	fmt.Print("email> ")
	fmt.Scanln(&email)
	kcore.Expect(emailInput.Fill(email), "failed to fill email input")
	passwordInput := page.Locator("input#password")
	kcore.Expect(passwordInput.Click(), "failed to click password input")
	var password string
	fmt.Print("password> ")
	fmt.Scanln(&password)
	kcore.Expect(passwordInput.Fill(password), "failed to fill password")
	submitButton := page.Locator("button")
	kcore.Expect(submitButton.Click(), "failed to click submit button")
}

type GarminImporter struct{}

func (i *GarminImporter) Run() {
	var email string
	fmt.Print("email> ")
	fmt.Scanln(&email)
	var password string
	fmt.Print("password> ")
	fmt.Scanln(&password)
	url := "https://sso.garmin.com/portal/api/login?clientId=GarminConnect&locale=en-GB&service=https://connect.garmin.com/modern"
	data := []byte(fmt.Sprintf(`{"username":"martin@lehoux.net","password":"ejbSZrQZU7HDcshP","rememberMe":false,"captchaToken":""}`))
	res, err := http.Post(url, "application/json", bytes.NewReader(data))
	kcore.Expect(err, "failed to post login")
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	fmt.Println(string(body))
}
