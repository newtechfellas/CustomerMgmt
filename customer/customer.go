package customer

type Customer struct {
	id               string
	VerificationCode string
	FirstName        string
	LastName         string
	Email            string
	PhoneNumber      string
	Address          struct {
				 line1 string
				 line2 string
				 city  string
				 state string
				 zip   int
			 }
}