package customer



import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	. "github.com/newtechfellas/GoUtil/util"
	"golang.org/x/net/context"
	"net/http"
	"strconv"
	"time"
	"log"
)

func NewCustomer(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	u, err := getCustomerFromReqPayload(ctx, w, r)
	if err != nil {
		return //error is already handled
	}
	u.VerificationCode = 0 //reset verification code sent by customer
	if u.exists(ctx) {
		//customer already exists
		log.Errorf(ctx, "Trying to register an existing customer %v", Jsonify(u))
		ErrorResponse(w, errors.New("Customer already exists with this phone number"), http.StatusBadRequest)
		return
	}
	log.Debugf(ctx, "Registering customer %v", Jsonify(u))
	u.CreatedTime = time.Now()
	u.VerificationCode = Random4DigitNumber()
	if err = u.save(ctx); err != nil {
		log.Errorf(ctx, "Error in registering a customer %v. Error is %v", Jsonify(u), err)
		ErrorResponse(w, errors.New("Error in registering customer"), http.StatusInternalServerError)
		return
	}
	//send sms to confirm the verification code
	//		if err = sendSms_Plivo(ctx, customer.PhoneNumber,customerConfirmSmsText(customer)); err != nil {
	if err = sendSms_Twilio(ctx, u.PhoneNumber, customerVerificationSmsText(u)); err != nil {
		u.delete(ctx)
		log.Errorf(ctx, "Error in sending sms to confirm customer %v. Error is %v ", Jsonify(u), err)
		ErrorResponse(w, errors.New("Error in registering customer"), http.StatusInternalServerError)
		return
	}
	SimpleJsonResponse(w, http.StatusCreated)
	return
}

func UpdateCustomer(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	customer, err := validateCustomerFromReqAgainstDB(ctx, w, r)
	if err != nil {
		return //error is already handled
	}
	if err = customer.save(ctx); err != nil {
		log.Errorf(ctx, "UpdateCustomer: Could not update Customer %v", Jsonify(customer))
		ErrorResponse(w, errors.New("Customer update failed"), http.StatusBadRequest)
		return
	}
	SimpleJsonResponse(w, http.StatusOK)
	return
}

func ConfirmCustomer(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	u, err := validateCustomerFromReqAgainstDB(ctx, w, r)
	if err != nil {
		return //error is already handled
	}
	//verification codes match. Confirm the customer
	log.Debugf(ctx, "customer to be confirmed %v", u.PhoneNumber)
	u.VerifiedTime = time.Now()
	if err := u.save(ctx); err != nil {
		log.Errorf(ctx, "Error in confirming customer %v. Error is %v", Jsonify(u), err)
		ErrorResponse(w, errors.New("Error in confirming customer "), http.StatusInternalServerError)
	} else {
		//Send the verification code as a header to be cached by the mobile client
		w.Header().Set("vc", strconv.Itoa(u.VerificationCode)) //do we need to encrypt it??
		SimpleJsonResponse(w, http.StatusOK)
	}
	return
}

func ReConfirmCustomer(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	u, err := validateCustomerFromReqAgainstDB(ctx, w, r)
	if err != nil {
		return //error is already handled
	}
	//Generate a new verification code and send sms
	log.Debugf(ctx, "customer to be confirmed %v", u.PhoneNumber)
	u.VerifiedTime = time.Time{}
	u.VerificationCode = Random4DigitNumber()
	if err = u.save(ctx); err != nil {
		log.Errorf(ctx, "Error in reconfirming customer %v. Error is %v", Jsonify(u), err)
		ErrorResponse(w, errors.New("Error in reconfirming customer"), http.StatusInternalServerError)
		return
	}
	//	send sms to confirm the verification code
	//			if err = sendSms_Plivo(ctx, customer.PhoneNumber,customerConfirmSmsText(customer)); err != nil {
	if err = sendSms_Twilio(ctx, u.PhoneNumber, customerVerificationSmsText(u)); err != nil {
		log.Errorf(ctx, "Error in sending sms to confirm customer %v. Error is %v ", Jsonify(u), err)
		ErrorResponse(w, errors.New("Error in reconfirming customer"), http.StatusInternalServerError)
		return
	}
	SimpleJsonResponse(w, http.StatusOK)
	return
}

func DeleteCustomer(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	customer, err := validateCustomerFromReqAgainstDB(ctx, w, r)
	if err != nil {
		return //error is already handled
	}
	if err = customer.delete(ctx); err != nil {
		log.Errorf(ctx, "Failed to delete the customer %v", Jsonify(customer))
		ErrorResponse(w, errors.New("Failed to delete the customer"), http.StatusInternalServerError)
		return
	}
	SimpleJsonResponse(w, http.StatusOK)
	return
}

func getCustomerFromReqPayload(ctx context.Context, w http.ResponseWriter, r *http.Request) (u Customer, err error) {
	decoder := json.NewDecoder(r.Body)
	vars := mux.Vars(r)
	//Do we have customer info in URI or RequestBody?
	phoneNumber := vars["phoneNumber"]
	verificationCode := vars["verificationCode"]
	if len(phoneNumber) > 0 && len(verificationCode) > 0 {
		u.PhoneNumber = phoneNumber
		u.VerificationCode, _ = strconv.Atoi(verificationCode)
	} else {
		//Checking if the request body has the customer details
		if err = decoder.Decode(&u); err != nil {
			log.Errorf(ctx, "getCustomerFromReqPayload: Error in decoding request body. Error is %v", err)
			ErrorResponse(w, errors.New("Invalid details in request"), http.StatusBadRequest)
			return
		}
	}
	log.Debugf(ctx, "getCustomerFromReqPayload: decoded customer: %v", u)
	if len(u.PhoneNumber) == 0 {
		log.Errorf(ctx, "PhoneNumber is mandatory")
		err = errors.New("PhoneNumber is mandatory")
		ErrorResponse(w, err, http.StatusBadRequest)
		return
	}
	return
}

func customerVerificationSmsText(p Customer) string {
	return fmt.Sprintf("Your verification code is %v", p.VerificationCode)
}

func getDBCustomerFromReq(ctx context.Context, w http.ResponseWriter, r *http.Request) (dbCustomer Customer, err error) {
	var u Customer
	u, err = getCustomerFromReqPayload(ctx, w, r)
	if err != nil {
		return //error is already handled
	}
	dbCustomer.PhoneNumber = u.PhoneNumber
	if err = dbCustomer.get(ctx); err == datastore.ErrNoSuchEntity {
		log.Errorf(ctx, "Customer does not exist %v", Jsonify(dbCustomer))
		SimpleJsonResponse(w, http.StatusNotFound) //intentionally do not return error
		return
	}
	if dbCustomer.VerificationCode != u.VerificationCode {
		log.Errorf(ctx, "Incoming customer has invalid verification code. This request is suspicious %v", Jsonify(u))
		err = errors.New("Incoming customer has invalid verification code")
		SimpleJsonResponse(w, http.StatusBadRequest) //intentionally do not return error
		return
	}
	return
}

func validateCustomerFromReqAgainstDB(ctx context.Context, w http.ResponseWriter, r *http.Request) (customer Customer, err error) {
	customer, err = getCustomerFromReqPayload(ctx, w, r)
	if err != nil {
		return //error is already handled
	}
	if customer.exists(ctx) == false {
		err = errors.New("Customer does not exist")
		log.Errorf(ctx, "Customer does not exist with given data %v", Jsonify(customer))
		SimpleJsonResponse(w, http.StatusNotFound) //intentionally do not return error
		return
	}
	return
}
