// Copyright 2018 Vector Creations Ltd
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
//
// Modifications copyright (C) 2020 Finogeeks Co., Ltd

package workers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/finogeeks/ligase/common"
	"math"
	"net/http"
	"time"

	"github.com/finogeeks/ligase/appservice/types"
	"github.com/finogeeks/ligase/common/config"
	"github.com/finogeeks/ligase/skunkworks/gomatrixserverlib"
	log "github.com/finogeeks/ligase/skunkworks/log"
	"github.com/finogeeks/ligase/storage/model"
)

var (
	// Maximum size of events sent in each transaction.
	transactionBatchSize = 50
	// Timeout for sending a single transaction to an application service.
	transactionTimeout = time.Second * 60
)

// SetupTransactionWorkers spawns a separate goroutine for each application
// service. Each of these "workers" handle taking all events intended for their
// app service, batch them up into a single transaction (up to a max transaction
// size), then send that off to the AS's /transactions/{txnID} endpoint. It also
// handles exponentially backing off in case the AS isn't currently available.
func SetupTransactionWorkers(
	appserviceDB model.AppServiceDatabase,
	workerStates []types.ApplicationServiceWorkerState,
) error {
	// Create a worker that handles transmitting events to a single homeserver
	for _, workerState := range workerStates {
		log.Infof("start workerState for %s", workerState.AppService.URL)
		// Don't create a worker if this AS doesn't want to receive events
		if workerState.AppService.URL != "" {
			go worker(appserviceDB, workerState)
		}
	}
	return nil
}

// worker is a goroutine that sends any queued events to the application service
// it is given.
func worker(db model.AppServiceDatabase, ws types.ApplicationServiceWorkerState) {
	log.Infow("starting application service", log.KeysAndValues{"appservice", ws.AppService.ID})
	span, ctx := common.StartSobSomSpan(context.Background(), "transaction_scheduler.worker")
	defer span.Finish()

	// Create a HTTP client for sending requests to app services
	client := &http.Client{
		Timeout: transactionTimeout,
	}

	// Initial check for any leftover events to send from last time
	eventCount, err := db.CountEventsWithAppServiceID(ctx, ws.AppService.ID)
	if err != nil {
		log.Fatalw("appservice worker unable to read queued events from DB", log.KeysAndValues{"appservice", ws.AppService.ID, "error", err})
		return
	}
	if eventCount > 0 {
		ws.NotifyNewEvents()
	}

	// Loop forever and keep waiting for more events to send
	for {
		// Wait for more events if we've sent all the events in the database
		ws.WaitForNewEvents()

		// Batch events up into a transaction
		transactionJSON, txnID, maxEventID, eventsRemaining, err := createTransaction(ctx, db, ws.AppService.ID)
		if err != nil {
			log.Fatalw("appservice worker unable to create transaction", log.KeysAndValues{"appservice", ws.AppService.ID, "error", err})

			return
		}

		// Send the events off to the application service
		// Backoff if the application service does not respond
		err = send(client, ws.AppService, txnID, transactionJSON)
		if err != nil {
			// Backoff
			backoff(&ws, err)
			continue
		}

		// We sent successfully, hooray!
		ws.Backoff = 0

		// Transactions have a maximum event size, so there may still be some events
		// left over to send. Keep sending until none are left
		if !eventsRemaining {
			ws.FinishEventProcessing()
		}

		// Remove sent events from the DB
		err = db.RemoveEventsBeforeAndIncludingID(ctx, ws.AppService.ID, maxEventID)
		if err != nil {
			log.Fatalw("unable to remove appservice events from the database", log.KeysAndValues{"appservice", ws.AppService.ID, "error", err})
			return
		}
	}
}

// backoff pauses the calling goroutine for a 2^some backoff exponent seconds
func backoff(ws *types.ApplicationServiceWorkerState, err error) {
	// Calculate how long to backoff for
	backoffDuration := time.Duration(math.Pow(2, float64(ws.Backoff)))
	backoffSeconds := time.Second * backoffDuration

	log.Warnw(fmt.Sprintf("unable to send transactions successfully, backing off for %ds", backoffDuration), log.KeysAndValues{
		"appservice", ws.AppService.ID, "error", err,
	})

	ws.Backoff++
	if ws.Backoff > 6 {
		ws.Backoff = 6
	}

	// Backoff
	time.Sleep(backoffSeconds)
}

// createTransaction takes in a slice of AS events, stores them in an AS
// transaction, and JSON-encodes the results.
func createTransaction(
	ctx context.Context,
	db model.AppServiceDatabase,
	appserviceID string,
) (
	transactionJSON []byte,
	txnID, maxID int,
	eventsRemaining bool,
	err error,
) {
	// Retrieve the latest events from the DB (will return old events if they weren't successfully sent)
	txnID, maxID, events, eventsRemaining, err := db.GetEventsWithAppServiceID(ctx, appserviceID, transactionBatchSize)
	if err != nil {
		log.Fatalw("appservice worker unable to read queued events from DB", log.KeysAndValues{"appservice", appserviceID, "error", err})

		return
	}

	// Check if these events do not already have a transaction ID
	if txnID == -1 {
		// If not, grab next available ID from the DB
		txnID, err = db.GetLatestTxnID(ctx)
		if err != nil {
			log.Fatalw("get latest txnid error", log.KeysAndValues{"appservice", appserviceID, "error", err})
			fmt.Println("get latest txnid error")
			return nil, 0, 0, false, err
		}

		// Mark new events with current transactionID
		if err = db.UpdateTxnIDForEvents(ctx, appserviceID, maxID, txnID); err != nil {
			log.Fatalw("update txnid error", log.KeysAndValues{"appservice", appserviceID, "error", err})
			fmt.Println("update txnid error")
			return nil, 0, 0, false, err
		}
	}

	// Create a transaction and store the events inside
	transaction := gomatrixserverlib.ApplicationServiceTransaction{
		Events: events,
	}

	transactionJSON, err = json.Marshal(transaction)
	if err != nil {
		return
	}

	return
}

// send sends events to an application service. Returns an error if an OK was not
// received back from the application service or the request timed out.
func send(
	client *http.Client,
	appservice config.ApplicationService,
	txnID int,
	transaction []byte,
) error {
	if txnID == 0 {
		return nil
	}

	// POST a transaction to our AS
	address := fmt.Sprintf("%s/transactions/%d?access_token=%s", appservice.URL, txnID, appservice.HSToken)

	req, err := http.NewRequest("PUT", address, bytes.NewBuffer(transaction))
	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)

	log.Infow("send requset "+address, log.KeysAndValues{"appservice", appservice.ID})

	if err != nil {
		return err
	}
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			log.Errorw("unable to close response body from application service", log.KeysAndValues{"appservice", appservice.ID, "error", err})
		}
	}()

	//Check the AS received the events correctly
	if resp.StatusCode != http.StatusOK {
		// TODO: Handle non-200 error codes from application services
		return fmt.Errorf("non-OK status code %d returned from AS", resp.StatusCode)
	}

	return nil
}
