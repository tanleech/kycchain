package main

import (
	"errors"
	"fmt"
	"encoding/json"
	"github.com/op/go-logging"
	"github.com/hyperledger/fabric/core/chaincode/shim"
)

var myLogger = logging.MustGetLogger("kyc_v2_records")

type KycChaincode struct {
}

type RecordList struct {
	NumEntries	int			`json:"numEntries"`
	Records		[]Record	`json:"records"`
}

type Record struct {
	RecId			string	`json:"recId"`
	CustId			string	`json:"custId"`
	CustName		string	`json:"custName"`
	RecOwner		string 	`json:"recOwner"`
	EncFor 			string 	`json:"encFor"`
	EncData			string 	`json:"encData"`
	DateCreated 	string	`json:"dateCreated"`
	DateModified	string	`json:"dateModified"`
}

func (t* KycChaincode) addRecord(stub *shim.ChaincodeStub, custId string, custName string, recOwner string, encFor string, encData string, dateCreated string) (error) {
	recId:=custId+recOwner+encFor
	
	
	resp,err:=stub.GetState(recId)
	if(err!=nil) {
		myLogger.Info("[addRequest] Error: %s" ,err)
		return err
	}
	if(resp!=nil) {
		myLogger.Info("[addRequest] Record [%s] already exists" ,recId)
		return errors.New("Record already exists")
	}
	
	
	newRec:=Record{
		RecId		:recId,
		CustId		:custId,
		CustName	:custName,
		RecOwner	:recOwner,
		EncFor		:encFor,
		EncData		:encData,
		DateCreated	:dateCreated,
		DateModified:dateCreated,
	}	
	
	newRecBytes,err:=json.Marshal(newRec)
	if(err!=nil) {
		myLogger.Info("[addRecord] Error: %s" ,err)
		return err
	}
	
	myLogger.Info("[addRecord] Adding Record: %s" ,newRec)
	
	err=stub.PutState(recId, newRecBytes)
	if(err!=nil) {
		myLogger.Info("[addRecord] Error: %s" ,err)
		return err
	}
	
	err=t.indexRecord(stub, newRec)
	if(err!=nil) {
		myLogger.Info("[addRecord] Error: %s" ,err)
		return err
	}
	
	return nil
}

func (t* KycChaincode) updateRecord(stub *shim.ChaincodeStub, recId string, encData string, dateModified string) (error) {
	
	var rec Record
	recBytes, err:= t.getRecord(stub, recId)
	if(err!=nil) {
		myLogger.Info("[updateRecord] Error: %s" ,err)
		return err
	}
	
	err=json.Unmarshal(recBytes, &rec)
	if(err!=nil) {
		myLogger.Info("[updateRecord] Error: %s" ,err)
		return err
	}
	
	err=t.updateIndex(stub, rec, dateModified)
	if(err!=nil) {
		myLogger.Info("[updateRecord] Error: %s" ,err)
		return err
	}
	
	
	rec.EncData = encData
	rec.DateModified = dateModified
	
	recBytes, err= json.Marshal(rec)
	
	err=stub.PutState(recId, recBytes)
	if(err!=nil) {
		myLogger.Info("[updateRecord] Error: %s" ,err)
		return err
	}
	

	return nil
}

func (t* KycChaincode) indexRecord(stub *shim.ChaincodeStub, rec Record) (error) {
	idKey 	:= "id"+rec.CustId+rec.RecId
	nameKey	:= "name"+rec.CustName+rec.RecId
	dateKey	:= "date"+rec.DateModified+rec.RecId
	myLogger.Info("[indexRecord] Indexing Record: [%s] - id: [%s], name: [%s], date [%s]" ,rec.RecId, idKey, nameKey, dateKey)
	
	recIdBytes:=[]byte(rec.RecId)
	
	err := stub.PutState(idKey, recIdBytes) 
	if err != nil {
		return errors.New("Failed adding to Id Index.")
	}	
	
	err = stub.PutState(nameKey, recIdBytes) 
	if err != nil {
		return errors.New("Failed adding to Name Index.")
	}
	
	err = stub.PutState(dateKey, recIdBytes) 
	if err != nil {
		return errors.New("Failed adding to Date Index.")
	}
	
	return nil
}

func (t* KycChaincode) updateIndex(stub *shim.ChaincodeStub, rec Record, newDateModified string) (error) {
	err := stub.DelState("date"+rec.DateModified+rec.RecId)
	if err != nil {
		return errors.New("Failed to remove old date index.")
	}
	err = stub.PutState("date"+newDateModified+rec.RecId, []byte(rec.RecId))
	if err != nil {
		return errors.New("Failed to add new date index.")
	}
	
	return nil
}

func (t* KycChaincode) getRecord(stub *shim.ChaincodeStub, recId string) ([]byte, error) {
	rec,err:=stub.GetState(recId)
	if(err!=nil) {
		myLogger.Info("[getRecord] Error: %s" ,err)
		return nil, err
	}
	if(rec==nil) {
		myLogger.Info("[getRecord] No record with recId [%s]" ,recId)
		return nil, errors.New("No record with given recId")
	}
	
	return rec, nil
}

func (t* KycChaincode) searchIndex(stub *shim.ChaincodeStub, input string, field string) ([]byte, error) {
	var indexToSearch string
	switch field {
		case "id": indexToSearch="id"
		case "name": indexToSearch="name"
		case "date": indexToSearch="date"
		default: return nil, errors.New("No such index to search")
	}

	keysIter, err := stub.RangeQueryState(indexToSearch+input, indexToSearch+input+"ZZZZZZZZZZZZZZZZZZZZZZ")
	if err != nil {
		return nil, errors.New("Failed adding to get keysIter.")
	}
	defer keysIter.Close()
	
	var results RecordList
	var currRec Record
	
	for keysIter.HasNext() {
		_, recId, err := keysIter.Next()
		recBytes,err:=t.getRecord(stub, string(recId))
		if err != nil {
			return nil, err
		}	
		err=json.Unmarshal(recBytes,&currRec)
		results.Records=append(results.Records, currRec)
		results.NumEntries++
		
		if err != nil {
			return nil, errors.New("Failed Iterating")
		}
		
	}
	
	resultsBytes,err:=json.Marshal(results)
	if err != nil {
		return nil, err
	}
	
	return resultsBytes, nil
		
}

func (t* KycChaincode) getRecordsAfterDate(stub *shim.ChaincodeStub, inputDate string) ([]byte, error) {
	keysIter, err := stub.RangeQueryState("date"+inputDate, "date"+"99999999999999999999999999")
	if err != nil {
		return nil, errors.New("Failed adding to get keysIter.")
	}
	defer keysIter.Close()
	
	myLogger.Info("[getRecordsAfterDate] Range from [%s] to [%s]" ,("date"+inputDate),("date"+"99999999999999999999999999"))

	var results RecordList
	var currRec Record
	
	for keysIter.HasNext() {
		_, recId, err := keysIter.Next()
		if err != nil {
			return nil, errors.New("Failed Iterating")
		}
		myLogger.Info("[getRecordsAfterDate] Handling Record [%s]" ,recId)
		
		recBytes,err:=t.getRecord(stub, string(recId))
		if err != nil {
			return nil, err
		}	
		err=json.Unmarshal(recBytes,&currRec)
		results.Records=append(results.Records, currRec)
		results.NumEntries++
	}
	
	resultsBytes,err:=json.Marshal(results)
	if err != nil {
		return nil, err
	}
	
	return resultsBytes, nil
}

func (t *KycChaincode) Init(stub *shim.ChaincodeStub, function string, args []string) ([]byte, error) {
	return nil,nil
}

func (t *KycChaincode) Invoke(stub *shim.ChaincodeStub, function string, args []string) ([]byte, error) {
	if(function=="addRec") {
		//expecting input in the form custId, custName, recOwner, encFor, encData, dateCreated
		err:=t.addRecord(stub, args[0], args[1], args[2], args[3], args[4], args[5])
		if (err!=nil) {
			return nil, err
		}	
		return nil, nil
	}
	
	if(function=="updateRec") {
		//expecting input in the form recId, encData, dateModified
		err:=t.updateRecord(stub, args[0], args[1], args[2])
		if (err!=nil) {
			return nil, err
		}	
		return nil, nil
	}
	
	return nil, errors.New("Function call not recognised")
}

func (t *KycChaincode) Query(stub *shim.ChaincodeStub, function string, args []string) ([]byte, error) {
	if(function=="recId") {
		rec, err:=t.getRecord(stub, args[0])
		if err != nil {
			return nil, err
		}
		return rec, nil 
	}
	
	if(function=="newRecs") {
		rec, err:=t.getRecordsAfterDate(stub, args[0])
		if err != nil {
			return nil, err
		}
		return rec, nil 
	}
	
	if(function=="searchIndex") {
		rec, err:=t.searchIndex(stub, args[0], args[1])
		if err != nil {
			return nil, err
		}
		return rec, nil 
	}
	
	
	return nil, errors.New("Function call not recognised")
}	

func main() {
	err := shim.Start(new(KycChaincode))
	if err != nil {
		fmt.Printf("Error starting KycChaincode: %s", err)
	}
}