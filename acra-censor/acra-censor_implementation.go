/*
Copyright 2018, Cossack Labs Limited

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package acracensor

import (
	"github.com/cossacklabs/acra/acra-censor/common"
	"github.com/cossacklabs/acra/acra-censor/handlers"
	log "github.com/sirupsen/logrus"
)

// ServiceName to use in logs
const ServiceName = "acra-censor"

// AcraCensor describes censor data: query handler, logger and reaction on parsing errors.
type AcraCensor struct {
	handlers              []QueryHandlerInterface
	ignoreParseError      bool
	unparsedQueriesWriter *common.QueryWriter
	logger                *log.Entry
}

// NewAcraCensor creates new censor object.
func NewAcraCensor() *AcraCensor {
	acraCensor := &AcraCensor{}
	acraCensor.logger = log.WithField("service", ServiceName)
	acraCensor.ignoreParseError = false
	return acraCensor
}

// AddHandler adds handler to the list of Censor handlers.
func (acraCensor *AcraCensor) AddHandler(handler QueryHandlerInterface) {
	acraCensor.handlers = append(acraCensor.handlers, handler)
}

// RemoveHandler removes handler from the list of Censor handlers.
func (acraCensor *AcraCensor) RemoveHandler(handler QueryHandlerInterface) {
	for index, handlerFromRange := range acraCensor.handlers {
		if handlerFromRange == handler {
			acraCensor.handlers = append(acraCensor.handlers[:index], acraCensor.handlers[index+1:]...)
		}
	}
}

// ReleaseAll stops all handlers.
func (acraCensor *AcraCensor) ReleaseAll() {
	acraCensor.logger = log.WithField("service", "acra-censor")
	acraCensor.ignoreParseError = false
	for _, handler := range acraCensor.handlers {
		handler.Release()
	}
}

// HandleQuery processes every query through each handler.
func (acraCensor *AcraCensor) HandleQuery(rawQuery string) error {
	if len(acraCensor.handlers) == 0 && acraCensor.unparsedQueriesWriter == nil {
		// no handlers, AcraCensor won't work
		return nil
	}
	normalizedQuery, queryWithHiddenValues, parsedQuery, err := common.HandleRawSQLQuery(rawQuery)
	// Unparsed query handling
	if err == common.ErrQuerySyntaxError {
		acraCensor.logger.WithError(err).Warning("Failed to parse input query")
		acraCensor.saveUnparsedQuery(rawQuery)
		if acraCensor.ignoreParseError {
			acraCensor.logger.Infoln("Unparsed query has been allowed")
			return nil
		}
		acraCensor.logger.Errorln("Unparsed query has been forbidden")
		return err
	}
	// Parsed query handling
	for _, handler := range acraCensor.handlers {
		if queryCaptureHandler, ok := handler.(*handlers.QueryCaptureHandler); ok {
			queryCaptureHandler.CheckQuery(queryWithHiddenValues, nil)
		}
		continueHandling, err := handler.CheckQuery(normalizedQuery, parsedQuery)
		if err != nil {
			acraCensor.logger.Errorf("Forbidden query: '%s'", queryWithHiddenValues)
			return err
		}
		//we don't have errors so allow query
		if !continueHandling {
			acraCensor.logger.Infof("Allowed query: '%s'", queryWithHiddenValues)
			return nil
		}
	}
	acraCensor.logger.Infof("Allowed query: '%s'", queryWithHiddenValues)
	return nil
}

func (acraCensor *AcraCensor) saveUnparsedQuery(query string) {
	if acraCensor.unparsedQueriesWriter != nil {
		acraCensor.unparsedQueriesWriter.WriteQuery(query)
	}
}
