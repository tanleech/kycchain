package main

import (
	"errors"
	"fmt"
	"encoding/json"
	"github.com/op/go-logging"
	"github.com/hyperledger/fabric/core/chaincode/shim"
)

var myLogger = logging.MustGetLogger("kyc_v2_requests")

type KycChaincode struct {
}

type RequestList struct {
	NumEntries	int			`json:"numEntries"`
	Requests	[]Request	`json:"requests"`
}

type Request struct {
	ReqId			string	`json:"reqId"`
	RecId			string	`json:"recId"`
	ReqBy			string	`json:"reqBy"`
	RecOwner		string 	`json:"recOwner"`
	Status 			string 	`json:"status"`
	DateCreated 	string	`json:"dateCreated"`
	DateModified	string	`json:"dateModified"`
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

func (t* KycChaincode) addRequest(stub *shim.ChaincodeStub, recId string, reqBy string, dateCreated string) (error) {
	reqId:=recId+reqBy+dateCreated
	recOwner, err:=t.getRecOwner(stub, recId)
	if(err!=nil) {
		myLogger.Info("[addRequest] Error: %s" ,err)
		return err
	}
	
	newReq:=Request{
		ReqId		:reqId,
		RecId		:recId,
		ReqBy		:reqBy,
		RecOwner	:recOwner,
		Status		:"Pending",
		DateCreated	:dateCreated,
		DateModified:dateCreated,
	}	
	
	newReqBytes,err:=json.Marshal(newReq)
	if(err!=nil) {
		myLogger.Info("[addRequest] Error: %s" ,err)
		return err
	}
	
	myLogger.Info("[addRequest] Adding Record: %s" ,newReq)
	
	err=stub.PutState(reqId, newReqBytes)
	if(err!=nil) {
		myLogger.Info("[addRequest] Error: %s" ,err)
		return err
	}
	
	err=t.indexRequest(stub, newReq)
	if(err!=nil) {
		myLogger.Info("[addRequest] Error: %s" ,err)
		return err
	}
	
	return nil
}

func (t* KycChaincode) indexRequest(stub *shim.ChaincodeStub, req Request) (error) {
	byKey 	:= "by"+req.ReqBy+req.ReqId
	forKey	:= "for"+req.RecOwner+req.ReqId
	dateKey	:= "date"+req.DateModified+req.RecId
	myLogger.Info("[indexRequest] Indexing Request: [%s] - id: [%s], name: [%s], date [%s]" ,req.ReqId, byKey, forKey, dateKey)
	
	reqIdBytes:=[]byte(req.ReqId)
	
	err := stub.PutState(byKey, reqIdBytes) 
	if err != nil {
		return errors.New("Failed adding to By Index.")
	}	
	
	err = stub.PutState(forKey, reqIdBytes) 
	if err != nil {
		return errors.New("Failed adding to For Index.")
	}
	
	err = stub.PutState(dateKey, reqIdBytes) 
	if err != nil {
		return errors.New("Failed adding to Date Index.")
	}
	
	return nil
}

func (t* KycChaincode) getRequest(stub *shim.ChaincodeStub, reqId string) ([]byte, error) {
	req,err:=stub.GetState(reqId)
	if(err!=nil) {
		myLogger.Info("[getRequest] Error: %s" ,err)
		return nil, err
	}
	if(req==nil) {
		myLogger.Info("[getRequest] No record with reqId [%s]" ,reqId)
		return nil, errors.New("No request with given req")
	}
	
	return req, nil
}

func (t* KycChaincode) searchIndex(stub *shim.ChaincodeStub, input string, field string) ([]byte, error) {
	var indexToSearch string
	switch field {
		case "by": indexToSearch="by"
		case "for": indexToSearch="for"
		case "date": indexToSearch="date"
		default: return nil, errors.New("No such index to search")
	}

	keysIter, err := stub.RangeQueryState(indexToSearch+input, indexToSearch+input+"ZZZZZZZZZZZZZZZZZZZZZZ")
	if err != nil {
		return nil, errors.New("Failed adding to get keysIter.")
	}
	defer keysIter.Close()
	
	var results RequestList
	var currReq Request
	
	for keysIter.HasNext() {
		_, reqId, err := keysIter.Next()
		reqBytes,err:=t.getRequest(stub, string(reqId))
		if err != nil {
			return nil, err
		}	
		err=json.Unmarshal(reqBytes,&currReq)
		results.Requests=append(results.Requests, currReq)
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

func (t* KycChaincode) getRequestsAfterDate(stub *shim.ChaincodeStub, inputDate string) ([]byte, error) {
	keysIter, err := stub.RangeQueryState("date"+inputDate, "date"+"99999999999999999999999999")
	if err != nil {
		return nil, errors.New("Failed adding to get keysIter.")
	}
	defer keysIter.Close()
	
	myLogger.Info("[getRequestsAfterDate] Range from [%s] to [%s]" ,("date"+inputDate),("date"+"99999999999999999999999999"))

	var results RequestList
	var currReq Request
	
	for keysIter.HasNext() {
		_, reqId, err := keysIter.Next()
		if err != nil {
			return nil, errors.New("Failed Iterating")
		}
		myLogger.Info("[getRequestsAfterDate] Handling Record [%s]" ,reqId)
		
		reqBytes,err:=t.getRequest(stub, string(reqId))
		if err != nil {
			return nil, err
		}	
		err=json.Unmarshal(reqBytes,&currReq)
		results.Requests=append(results.Requests, currReq)
		results.NumEntries++
	}
	
	resultsBytes,err:=json.Marshal(results)
	if err != nil {
		return nil, err
	}
	
	return resultsBytes, nil
}

func (t* KycChaincode) getRecOwner(stub *shim.ChaincodeStub, recId string) (string, error) {
	recBytes, err:=stub.QueryChaincode("mycc","recId",[]string{recId})
	if(err!=nil) {
		myLogger.Info("[getRecOwner] Error: %s" ,err)
		return "err", err
	}
	
	var rec Record
	err=json.Unmarshal(recBytes, &rec)
	if(err!=nil) {
		myLogger.Info("[getRecOwner] Error: %s" ,err)
		return "err", err
	}
	
	return rec.RecOwner,nil

}

func (t* KycChaincode) giveAccess(stub *shim.ChaincodeStub, reqId string, encData string, dateModified string) (error) {
	reqBytes, err:=t.getRequest(stub, reqId)
	if(err!=nil) {
		myLogger.Info("[giveAccess] Error: %s" ,err)
		return err
	}
	
	var req Request
	err=json.Unmarshal(reqBytes, &req)
	if(err!=nil) {
		myLogger.Info("[giveAccess] Error: %s" ,err)
		return err
	}
	
	recBytes, err:=stub.QueryChaincode("mycc","recId",[]string{req.RecId})
	if(err!=nil) {
		myLogger.Info("[giveAccess] Error: %s" ,err)
		return err
	}
	
	var rec Record
	err=json.Unmarshal(recBytes, &rec)
	if(err!=nil) {
		myLogger.Info("[giveAccess] Error: %s" ,err)
		return err
	}
	
	
	//SOME VERIFICATION HERE, TO CONFIRM PERSON GIVING ACCESS IS INDEED RECORD OWNER
	
	_, err=stub.InvokeChaincode("mycc", "addRec", []string{rec.CustId, rec.CustName, req.RecOwner, req.ReqBy, encData, dateModified})
	if(err!=nil) {
		myLogger.Info("[giveAccess] Error: %s" ,err)
		return err
	}
	//TO MOVE TO NEW FUNC?
	req.Status="Access Given on "+dateModified
	req.DateModified=dateModified
	
	reqBytes,err=json.Marshal(req)
	if(err!=nil) {
		myLogger.Info("[giveAccess] Error: %s" ,err)
		return err
	}
	
	myLogger.Info("[giveAccess] Updating Record: %s" ,req)
	
	err=stub.PutState(req.ReqId, reqBytes)
	if(err!=nil) {
		myLogger.Info("[addRequest] Error: %s" ,err)
		return err
	}
	
	return nil
}

func (t *KycChaincode) Init(stub *shim.ChaincodeStub, function string, args []string) ([]byte, error) {
	return nil,nil
}

func (t *KycChaincode) Invoke(stub *shim.ChaincodeStub, function string, args []string) ([]byte, error) {
	if(function=="addReq") {
		//expecting input in the form recId, reqBy, dateCreated
		err:=t.addRequest(stub, args[0], args[1], args[2])
		if (err!=nil) {
			return nil, err
		}	
		return nil, nil
	}
	
	if(function=="giveAccess") {
		//expecting input in the form reqId, encData, dateModified
		err:=t.giveAccess(stub, args[0], args[1], args[2])
		if (err!=nil) {
			return nil, err
		}	
		return nil, nil
	}
	
	return nil, errors.New("Function call not recognised")
}

func (t *KycChaincode) Query(stub *shim.ChaincodeStub, function string, args []string) ([]byte, error) {
	if(function=="reqId") {
		rec, err:=t.getRequest(stub, args[0])
		if err != nil {
			return nil, err
		}
		return rec, nil 
	}
	
	if(function=="newReqs") {
		rec, err:=t.getRequestsAfterDate(stub, args[0])
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