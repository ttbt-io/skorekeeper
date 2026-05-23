package e2e

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

func TestSpeechInput(t *testing.T) {
	if *withChromeDP == "" {
		t.Skip("--with-chromedp not set")
	}

	baseURL := startTestServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	allocCtx, cancel := chromedp.NewRemoteAllocator(ctx, *withChromeDP)
	defer cancel()

	opts := []chromedp.ContextOption{
		chromedp.WithLogf(log.Printf),
		chromedp.WithErrorf(log.Printf),
	}
	ctx, cancel = chromedp.NewContext(allocCtx, opts...)
	defer cancel()

	// Capture JS console errors
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		// Log errors to surface the "JS exception"
	})

	var gameID string

	runStep(t, ctx, "Create Game",
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := Login(ctx, baseURL); err != nil {
				return err
			}
			return nil
		}),
		chromedp.WaitVisible(`#dashboard-view:not(.hidden)`),
		chromedp.WaitVisible(`#btn-new-game`),
		chromedp.Click(`#btn-new-game`),
		chromedp.WaitEnabled(`input#team-away-input`),
		chromedp.SetValue(`input#team-away-input`, "Away"), chromedp.SetValue(`input#team-home-input`, "Home"),
		chromedp.Click(`#btn-start-new-game`),
		// Wait for transition to scoresheet
		chromedp.WaitVisible(`#scoresheet-view:not(.hidden)`),
		chromedp.Evaluate(`window.location.hash.substring(6)`, &gameID),
	)

	runStep(t, ctx, "Inject Mock SpeechRecognition",
		chromedp.Evaluate(`
			window.mockSpeechCallback = null;
			window.SpeechRecognition = class MockSpeechRecognition {
				constructor() {
					this.continuous = false;
					this.interimResults = false;
					this.lang = 'en-US';
				}
				start() {
					console.log("Mock SpeechRecognition started");
					window.mockSpeechCallback = this.onresult;
				}
				stop() {
					console.log("Mock SpeechRecognition stopped");
					if (this.onend) this.onend();
				}
			};
			window.webkitSpeechRecognition = window.SpeechRecognition;
		`, nil),
	)

	runStep(t, ctx, "Re-init SpeechManager so it picks up the mock",
		// We need to re-instantiate SpeechManager or at least make AppController pick up the mock.
		// Since it checks `window.SpeechRecognition` in the constructor, we can just replace the instance.
		chromedp.Evaluate(`
			window.app.speechManager = new window.app.speechManager.constructor(() => window.app.state.activeGame);
		`, nil),
	)

	runStep(t, ctx, "Test Speech from Scoresheet Header",
		chromedp.WaitVisible(`#btn-speech-toggle`),
		chromedp.Click(`#btn-speech-toggle`),
		chromedp.Evaluate(`window.mockSpeechCallback({ results: [[{ transcript: "strike" }]] })`, nil),
		chromedp.WaitVisible(`#speech-preview-bar:not(.hidden)`),
		chromedp.WaitVisible(`#speech-action-chips`),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var chipsText string
			chromedp.Evaluate(`document.getElementById('speech-action-chips').textContent`, &chipsText).Do(ctx)
			t.Logf("Chips: %s", chipsText)
			return nil
		}),
		chromedp.Click(`#btn-speech-confirm`),
		chromedp.WaitNotVisible(`#speech-preview-bar`),
	)

	runStep(t, ctx, "Open CSO",
		chromedp.Click(`.grid-cell[data-key="away-0-col-1-0"]`),
		chromedp.WaitVisible(`#cso-modal:not(.hidden)`),
	)

	// In CSO, there is currently no speech button, but we can call it directly to see if it works when CSO is open
	// Or we wait for the fix that adds the button to CSO.
	runStep(t, ctx, "Test Speech from CSO (Direct Invocation for now)",
		chromedp.Evaluate(`window.app.toggleSpeech()`, nil),
		chromedp.Evaluate(`window.mockSpeechCallback({ results: [[{ transcript: "single" }]] })`, nil),
		chromedp.WaitVisible(`#speech-preview-bar:not(.hidden)`),
		chromedp.Click(`#btn-speech-confirm`),
		chromedp.WaitNotVisible(`#speech-preview-bar`),
	)
}
