package salesforce

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/mitchellh/mapstructure"
)

type sObjectCollection struct {
	AllOrNone string           `json:"allOrNone"`
	Records   []map[string]any `json:"records"`
}

func convertToMap(obj any) (map[string]any, error) {
	var recordMap map[string]any
	if _, ok := obj.(map[string]any); ok {
		recordMap = obj.(map[string]any)
	} else {
		err := mapstructure.Decode(obj, &recordMap)
		if err != nil {
			return nil, errors.New("issue decoding salesforce object, need a key value pair (custom struct or map)")
		}
	}
	return recordMap, nil
}

func convertToSliceOfMaps(obj any) ([]map[string]any, error) {
	var recordMap []map[string]any
	if _, ok := obj.(map[string]any); ok {
		recordMap = obj.([]map[string]any)
	} else {
		err := mapstructure.Decode(obj, &recordMap)
		if err != nil {
			return nil, errors.New("issue decoding salesforce object, need a key value pair (custom struct or map)")
		}
	}
	return recordMap, nil
}

func doInsertOne(auth Auth, sObjectName string, record any) error {
	recordMap, err := convertToMap(record)
	if err != nil {
		return err
	}
	recordMap["attributes"] = map[string]string{"type": sObjectName}
	delete(recordMap, "Id")

	body, err := json.Marshal(recordMap)
	if err != nil {
		return err
	}

	resp, err := doRequest("POST", "/sobjects/"+sObjectName, jsonType, auth, string(body))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return processSalesforceError(*resp)
	}

	return nil
}

func doUpdateOne(auth Auth, sObjectName string, record any) error {
	recordMap, err := convertToMap(record)
	if err != nil {
		return err
	}

	recordId, ok := recordMap["Id"].(string)
	if !ok || recordId == "" {
		return errors.New("salesforce id not found in object data")
	}

	recordMap["attributes"] = map[string]string{"type": sObjectName}
	delete(recordMap, "Id")

	body, err := json.Marshal(recordMap)
	if err != nil {
		return err
	}

	resp, err := doRequest("PATCH", "/sobjects/"+sObjectName+"/"+recordId, jsonType, auth, string(body))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusNoContent {
		return processSalesforceError(*resp)
	}

	return nil
}

func doUpsertOne(auth Auth, sObjectName string, fieldName string, record any) error {
	recordMap, err := convertToMap(record)
	if err != nil {
		return err
	}

	externalIdValue, ok := recordMap[fieldName].(string)
	if !ok || externalIdValue == "" {
		return errors.New("salesforce externalId: " + fieldName + " not found in " + sObjectName + " data. make sure to append custom fields with '__c'")
	}

	recordMap["attributes"] = map[string]string{"type": sObjectName}
	delete(recordMap, "Id")
	delete(recordMap, fieldName)

	body, err := json.Marshal(recordMap)
	if err != nil {
		return err
	}

	resp, err := doRequest("PATCH", "/sobjects/"+sObjectName+"/"+fieldName+"/"+externalIdValue, jsonType, auth, string(body))
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return processSalesforceError(*resp)
	}

	return nil
}

func doDeleteOne(auth Auth, sObjectName string, record any) error {
	recordMap, err := convertToMap(record)
	if err != nil {
		return err
	}

	recordId, ok := recordMap["Id"].(string)
	if !ok || recordId == "" {
		return errors.New("salesforce id not found in object data")
	}

	resp, err := doRequest("DELETE", "/sobjects/"+sObjectName+"/"+recordId, jsonType, auth, "")
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusNoContent {
		return processSalesforceError(*resp)
	}

	return nil
}

func doInsertCollection(auth Auth, sObjectName string, records any, allOrNone bool) error {
	recordMap, err := convertToSliceOfMaps(records)
	if err != nil {
		return err
	}

	if len(recordMap) > 200 {
		return errors.New("salesforce composite api call supports up to 200 records at once")
	}

	for i := range recordMap {
		delete(recordMap[i], "Id")
		recordMap[i]["attributes"] = map[string]string{"type": sObjectName}
	}

	payload := sObjectCollection{
		AllOrNone: strconv.FormatBool(allOrNone),
		Records:   recordMap,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := doRequest("POST", "/composite/sobjects/", jsonType, auth, string(body))
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return processSalesforceError(*resp)
	}
	salesforceErrors := processSalesforceResponse(*resp)
	if salesforceErrors != nil {
		return salesforceErrors
	}

	return nil
}

func doUpdateCollection(auth Auth, sObjectName string, records any, allOrNone bool) error {
	recordMap, err := convertToSliceOfMaps(records)
	if err != nil {
		return err
	}

	if len(recordMap) > 200 {
		return errors.New("salesforce composite api call supports up to 200 records at once")
	}

	for i := range recordMap {
		recordMap[i]["attributes"] = map[string]string{"type": sObjectName}
		recordId, ok := recordMap[i]["Id"].(string)
		if !ok || recordId == "" {
			return errors.New("salesforce id not found in object data")
		}
	}

	payload := sObjectCollection{
		AllOrNone: strconv.FormatBool(allOrNone),
		Records:   recordMap,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := doRequest("PATCH", "/composite/sobjects/", jsonType, auth, string(body))
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return processSalesforceError(*resp)
	}
	salesforceErrors := processSalesforceResponse(*resp)
	if salesforceErrors != nil {
		return salesforceErrors
	}

	return nil
}

func doUpsertCollection(auth Auth, sObjectName string, fieldName string, records any, allOrNone bool) error {
	recordMap, err := convertToSliceOfMaps(records)
	if err != nil {
		return err
	}

	if len(recordMap) > 200 {
		return errors.New("salesforce composite api call supports up to 200 records at once")
	}

	for i := range recordMap {
		recordMap[i]["attributes"] = map[string]string{"type": sObjectName}
		externalIdValue, ok := recordMap[i][fieldName].(string)
		if !ok || externalIdValue == "" {
			return errors.New("salesforce externalId: " + fieldName + " not found in " + sObjectName + " data. make sure to append custom fields with '__c'")
		}
	}

	payload := sObjectCollection{
		AllOrNone: strconv.FormatBool(allOrNone),
		Records:   recordMap,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := doRequest("PATCH", "/composite/sobjects/"+sObjectName+"/"+fieldName, jsonType, auth, string(body))
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return processSalesforceError(*resp)
	}
	salesforceErrors := processSalesforceResponse(*resp)
	if salesforceErrors != nil {
		return salesforceErrors
	}

	return nil
}

func doDeleteCollection(auth Auth, sObjectName string, records any, allOrNone bool) error {
	recordMap, err := convertToSliceOfMaps(records)
	if err != nil {
		return err
	}

	if len(recordMap) > 200 {
		return errors.New("salesforce composite api call supports up to 200 records at once")
	}

	var ids string
	for i := 0; i < len(recordMap); i++ {
		recordId, ok := recordMap[i]["Id"].(string)
		if !ok || recordId == "" {
			return errors.New("salesforce id not found in object data")
		}
		if i == len(recordMap)-1 {
			ids = ids + recordId
		} else {
			ids = ids + recordId + ","
		}
	}

	resp, err := doRequest("DELETE", "/composite/sobjects/?ids="+ids+"&allOrNone="+strconv.FormatBool(allOrNone), jsonType, auth, "")
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return processSalesforceError(*resp)
	}
	salesforceErrors := processSalesforceResponse(*resp)
	if salesforceErrors != nil {
		return salesforceErrors
	}

	return nil
}