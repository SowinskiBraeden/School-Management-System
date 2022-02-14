package controllers

import (
	"context"
	"os"
	"time"

	"github.com/SowinskiBraeden/school-management-api/database"
	"github.com/SowinskiBraeden/school-management-api/models"

	"net/smtp"

	"github.com/dgrijalva/jwt-go"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var teacherCollection *mongo.Collection = database.OpenCollection(database.Client, "teachers")
var studentCollection *mongo.Collection = database.OpenCollection(database.Client, "students")
var contactCollection *mongo.Collection = database.OpenCollection(database.Client, "contacts")
var adminCollection *mongo.Collection = database.OpenCollection(database.Client, "admins")

const SecretKey = "secret"

var systemEmail string = os.Getenv("SYSTEM_EMAIL")
var systemPassword string = os.Getenv("SYSTEM_PASSWORD")

func AuthAdmin(c *fiber.Ctx) bool {
	cookie := c.Cookies("jwt")

	token, err := jwt.ParseWithClaims(cookie, &jwt.StandardClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(SecretKey), nil
	})
	if err != nil {
		return false
	}

	claims := token.Claims.(*jwt.StandardClaims)

	// Though admin is not used, it's required to prevent findErr
	var admin models.Admin
	findErr := adminCollection.FindOne(context.TODO(), bson.M{"aid": claims.Issuer}).Decode(&admin)
	if findErr != nil {
		return false
	}

	return true
}

func AuthStudent(c *fiber.Ctx) (verified bool, sid string) {
	cookie := c.Cookies("jwt")

	token, err := jwt.ParseWithClaims(cookie, &jwt.StandardClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(SecretKey), nil
	})
	if err != nil {
		return false, ""
	}

	claims := token.Claims.(*jwt.StandardClaims)

	// Though student is not used, it's required to prevent findErr
	var student models.Student
	findErr := studentCollection.FindOne(context.TODO(), bson.M{"sid": claims.Issuer}).Decode(&student)
	if findErr != nil {
		return false, ""
	}

	return true, claims.Issuer
}

func Enroll(c *fiber.Ctx) error {
	var data map[string]interface{}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	if err := c.BodyParser(&data); err != nil {
		cancel()
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Failed to parse body",
			"error":   err,
		})
	}

	// Ensure Authenticated admin sent request
	if !AuthAdmin(c) {
		cancel()
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "Unauthorized: only an admin can perform this action",
		})
	}

	// Check minimum enroll field requirements are met
	if data["firstname"] == nil || data["lastname"] == nil || data["age"] == nil || data["gradelevel"] == nil || data["dob"] == nil || data["email"] == nil || data["province"] == nil || data["city"] == nil || data["address"] == nil || data["postal"] == nil {
		cancel()
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "missing required fields",
		})
	}

	var student models.Student
	student.PersonalData.FirstName = data["firstname"].(string)
	student.PersonalData.MiddleName = data["middlename"].(string)
	student.PersonalData.LastName = data["lastname"].(string)
	student.PersonalData.Age = data["age"].(float64)
	student.SchoolData.GradeLevel = data["gradelevel"].(float64)
	student.PersonalData.DOB = data["dob"].(string)
	student.PersonalData.Email = data["email"].(string)
	student.PersonalData.Province = data["province"].(string)
	student.PersonalData.City = data["city"].(string)
	student.PersonalData.Address = data["address"].(string)
	student.PersonalData.Postal = data["postal"].(string)
	student.PersonalData.Contacts = []string{}
	student.SchoolData.YOG = ((12 - int(student.SchoolData.GradeLevel)) + time.Now().Year()) + 1

	var photo models.Photo
	photo.Name = uuid.New().String()
	photo.Created_at, _ = time.Parse(time.RFC3339, time.Now().Format(time.RFC3339))
	photo.Updated_at, _ = time.Parse(time.RFC3339, time.Now().Format(time.RFC3339))
	photo.ID = primitive.NewObjectID()

	defaultImage, _ := os.ReadFile("./database/defaultImage.txt")
	photo.Base64 = string(defaultImage)

	student.SchoolData.PhotoName = photo.Name

	var schoolEmail string = ""
	offset := 0
	for {
		schoolEmail = student.GenerateSchoolEmail(offset, schoolEmail)
		if !student.EmailExists(schoolEmail) {
			break
		}
		offset++
	}
	student.AccountData.SchoolEmail = schoolEmail
	student.AccountData.HashHistory = []string{}

	// Disable login block
	student.AccountData.AccountDisabled = false
	student.AccountData.Attempts = 0

	// Generate temporary password
	var tempPass string = student.GeneratePassword(12, 1, 1, 1)
	student.AccountData.Password = student.HashPassword(tempPass)
	student.AccountData.TempPassword = true

	// Send student personal email temp password
	message := []byte("Your temporary password is " + tempPass)
	auth := smtp.PlainAuth("", systemEmail, systemPassword, "smtp.gmail.com")

	err := smtp.SendMail("smtp.gmail.com:587", auth, systemEmail, []string{student.PersonalData.Email}, message)
	if err != nil {
		cancel()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Could not send password to students email",
			"error":   err,
		})
	}

	var sid string
	for {
		sid = GenerateID(6)
		if ValidateID(sid) == true {
			break
		}
	}
	student.SchoolData.SID = sid

	var pen string
	for {
		pen = GenerateID(9)
		if ValidatePEN(pen) == true {
			break
		}
	}
	student.SchoolData.PEN = pen

	student.Created_at, _ = time.Parse(time.RFC3339, time.Now().Format(time.RFC3339))
	student.Updated_at, _ = time.Parse(time.RFC3339, time.Now().Format(time.RFC3339))
	student.ID = primitive.NewObjectID()

	_, insertErr := studentCollection.InsertOne(ctx, student)
	if insertErr != nil {
		cancel()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "the student could not be inserted",
			"error":   insertErr,
		})
	}

	_, insertErr = imageCollection.InsertOne(ctx, photo)
	if insertErr != nil {
		cancel()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "the student default photo could not be inserted",
			"error":   insertErr,
		})
	}
	defer cancel()

	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
		"success": true,
		"message": "successfully inserted student",
	})
}

func RegisterTeacher(c *fiber.Ctx) error {
	var data map[string]interface{}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	if err := c.BodyParser(&data); err != nil {
		cancel()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to parse body",
			"error":   err,
		})
	}

	// Ensure Authenticated admin sent request
	if !AuthAdmin(c) {
		cancel()
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "Unauthorized: only an admin can perform this action",
		})
	}

	// Check minimum register teacher field requirements are met
	if data["firstname"] == nil || data["lastname"] == nil || data["dob"] == nil || data["email"] == nil {
		cancel()
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "missing required fields",
		})
	}

	var teacher models.Teacher
	teacher.PersonalData.FirstName = data["firstname"].(string)
	teacher.PersonalData.MiddleName = data["middlename"].(string)
	teacher.PersonalData.LastName = data["lastname"].(string)
	teacher.PersonalData.Email = data["email"].(string)
	teacher.PersonalData.Province = data["province"].(string)
	teacher.PersonalData.City = data["city"].(string)
	teacher.PersonalData.Postal = data["postal"].(string)
	teacher.PersonalData.DOB = data["postal"].(string)

	var photo models.Photo
	photo.Name = uuid.New().String()
	photo.Created_at, _ = time.Parse(time.RFC3339, time.Now().Format(time.RFC3339))
	photo.Updated_at, _ = time.Parse(time.RFC3339, time.Now().Format(time.RFC3339))
	photo.ID = primitive.NewObjectID()

	defaultImage, _ := os.ReadFile("./database/defaultImage.txt")
	photo.Base64 = string(defaultImage)

	teacher.SchoolData.PhotoName = photo.Name

	var schoolEmail string = ""
	offset := 0
	for {
		schoolEmail = teacher.GenerateSchoolEmail(offset, schoolEmail)
		if !teacher.EmailExists(schoolEmail) {
			break
		}
		offset++
	}
	teacher.AccountData.SchoolEmail = schoolEmail
	teacher.AccountData.HashHistory = []string{}

	// Disable login block
	teacher.AccountData.AccountDisabled = false
	teacher.AccountData.Attempts = 0

	var tempPass string = teacher.GeneratePassword(12, 1, 1, 1)
	teacher.AccountData.Password = teacher.HashPassword(tempPass)
	teacher.AccountData.TempPassword = true

	// Send teacher personal email temp password
	message := []byte("Your temporary password is " + tempPass)
	auth := smtp.PlainAuth("", systemEmail, systemPassword, "smtp.gmail.com")

	err := smtp.SendMail("smtp.gmail.com:587", auth, systemEmail, []string{teacher.PersonalData.Email}, message)
	if err != nil {
		cancel()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Could not send password to teacners email",
			"error":   err,
		})
	}

	var tid string
	// For the unlikely event that an ID is already in use this will simply try again till it gets a id not in use
	for {
		tid = GenerateID(6)
		isValid := ValidateID(tid)
		if isValid == true {
			break
		}
	}
	teacher.SchoolData.TID = tid

	teacher.Created_at, _ = time.Parse(time.RFC3339, time.Now().Format(time.RFC3339))
	teacher.Updated_at, _ = time.Parse(time.RFC3339, time.Now().Format(time.RFC3339))
	teacher.ID = primitive.NewObjectID()

	_, insertErr := teacherCollection.InsertOne(ctx, teacher)
	if insertErr != nil {
		cancel()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "the teacher could not be inserted",
			"error":   insertErr,
		})
	}
	defer cancel()

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "successfully inserted teacher",
	})
}

func CreateAdmin(c *fiber.Ctx) error {
	var data map[string]interface{}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	if err := c.BodyParser(&data); err != nil {
		cancel()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to parse body",
			"error":   err,
		})
	}

	// Ensure Authenticated admin sent request
	if !AuthAdmin(c) {
		cancel()
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "Unauthorized: only an admin can perform this action",
		})
	}

	// Check minimum register teacher field requirements are met
	if data["firstname"] == nil || data["lastname"] == nil || data["dob"] == nil || data["email"] == nil {
		cancel()
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "missing required fields",
		})
	}

	var admin models.Admin
	admin.FirstName = data["firstname"].(string)
	admin.LastName = data["lastname"].(string)
	admin.Email = data["email"].(string)

	admin.SchoolEmail = admin.GenerateSchoolEmail()

	tempPass := admin.GeneratePassword(12, 1, 1, 1)
	admin.Password = admin.HashPassword(tempPass)
	admin.TempPassword = true

	// Send admin personal email temp password
	message := []byte("Your temporary password is " + tempPass)
	auth := smtp.PlainAuth("", systemEmail, systemPassword, "smtp.gmail.com")

	err := smtp.SendMail("smtp.gmail.com:587", auth, systemEmail, []string{admin.Email}, message)
	if err != nil {
		cancel()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Could not send password to admins email",
			"error":   err,
		})
	}

	var aid string
	for {
		aid = GenerateID(6)
		if ValidateID(aid) == true {
			break
		}
	}
	admin.AID = aid

	admin.Created_at, _ = time.Parse(time.RFC3339, time.Now().Format(time.RFC3339))
	admin.Updated_at, _ = time.Parse(time.RFC3339, time.Now().Format(time.RFC3339))
	admin.ID = primitive.NewObjectID()

	_, insertErr := adminCollection.InsertOne(ctx, admin)
	if insertErr != nil {
		cancel()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "the admin could not be inserted",
			"error":   insertErr,
		})
	}
	defer cancel()

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "successfully inserted admin",
	})
}

func StudentLogin(c *fiber.Ctx) error {
	var data map[string]string
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	if err := c.BodyParser(&data); err != nil {
		cancel()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to parse body",
			"error":   err,
		})
	}

	// Check required fields are included
	if data["sid"] == "" || data["password"] == "" {
		cancel()
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "missing required fields",
		})
	}

	var student models.Student
	err := studentCollection.FindOne(ctx, bson.M{"schooldata.sid": data["sid"]}).Decode(&student)
	defer cancel()

	if err != nil {
		cancel()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "student not found",
			"error":   err,
		})
	}

	var localAccountDisabled = false
	if student.AccountData.Attempts >= 5 {
		localAccountDisabled = true // Catches newly disbaled account before student obj is updated
		update_time, _ := time.Parse(time.RFC3339, time.Now().Format(time.RFC3339))
		update := bson.M{
			"$set": bson.M{
				"AccountData.accountdisabled": true,
				"AccountData.attempts":        0,
				"updated_at":                  update_time,
			},
		}

		_, updateErr := studentCollection.UpdateOne(
			ctx,
			bson.M{"sid": data["sid"]},
			update,
		)
		if updateErr != nil {
			cancel()
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "the student could not be updated",
				"error":   updateErr,
			})
		}
	}

	if localAccountDisabled || student.AccountData.AccountDisabled {
		cancel()
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"success": false,
			"message": "Account is Disabled, contact an Admin",
		})
	}

	var verified bool = student.ComparePasswords(data["password"])
	if verified == false {
		update_time, _ := time.Parse(time.RFC3339, time.Now().Format(time.RFC3339))
		update := bson.M{
			"$set": bson.M{
				"AccountData.attempts": student.AccountData.Attempts + 1,
				"updated_at":           update_time,
			},
		}

		_, updateErr := studentCollection.UpdateOne(
			ctx,
			bson.M{"sid": data["sid"]},
			update,
		)
		cancel()
		if updateErr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "the student could not be updated",
				"error":   updateErr,
			})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "incorrect password",
		})
	}
	defer cancel()

	claims := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.StandardClaims{
		Issuer:    student.SchoolData.SID,
		ExpiresAt: time.Now().Add(time.Hour * 24).Unix(), // 1 Day
	})
	token, err := claims.SignedString([]byte(SecretKey))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "could not log in",
		})
	}

	cookie := fiber.Cookie{
		Name:     "jwt",
		Value:    token,
		Expires:  time.Now().Add(time.Hour * 24),
		HTTPOnly: true,
	}
	c.Cookie(&cookie)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "correct password",
	})
}

func TeacherLogin(c *fiber.Ctx) error {
	var data map[string]string
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	if err := c.BodyParser(&data); err != nil {
		cancel()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to parse body",
			"error":   err,
		})
	}

	// Check required fields are included
	if data["tid"] == "" || data["password"] == "" {
		cancel()
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "missing required fields",
		})
	}

	var teacher models.Teacher
	err := teacherCollection.FindOne(ctx, bson.M{"schooldata.tid": data["tid"]}).Decode(&teacher)
	defer cancel()

	if err != nil {
		cancel()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "student not found",
			"error":   err,
		})
	}
	defer cancel()

	var verified bool = teacher.ComparePasswords(data["password"])
	if verified == false {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "incorrect password",
		})
	}

	claims := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.StandardClaims{
		Issuer:    teacher.SchoolData.TID,
		ExpiresAt: time.Now().Add(time.Hour * 24).Unix(), // 1 Day
	})
	token, err := claims.SignedString([]byte(SecretKey))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "could not log in",
		})
	}

	cookie := fiber.Cookie{
		Name:     "jwt",
		Value:    token,
		Expires:  time.Now().Add(time.Hour * 24),
		HTTPOnly: true,
	}
	c.Cookie(&cookie)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "correct password",
	})
}

func AdminLogin(c *fiber.Ctx) error {
	var data map[string]string
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	if err := c.BodyParser(&data); err != nil {
		cancel()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to parse body",
			"error":   err,
		})
	}

	// Check required fields are included
	if data["aid"] == "" || data["password"] == "" {
		cancel()
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "missing required fields",
		})
	}

	var admin models.Admin
	err := adminCollection.FindOne(ctx, bson.M{"aid": data["aid"]}).Decode(&admin)
	defer cancel()

	if err != nil {
		cancel()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "admin not found",
			"error":   err,
		})
	}
	defer cancel()

	var verified bool = admin.ComparePasswords(data["password"])
	if verified == false {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "incorrect password",
		})
	}

	claims := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.StandardClaims{
		Issuer:    admin.AID,
		ExpiresAt: time.Now().Add(time.Hour * 24).Unix(), // 1 Day
	})
	token, err := claims.SignedString([]byte(SecretKey))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "could not log in",
		})
	}

	cookie := fiber.Cookie{
		Name:     "jwt",
		Value:    token,
		Expires:  time.Now().Add(time.Hour * 24),
		HTTPOnly: true,
	}
	c.Cookie(&cookie)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "correct password",
	})
}

func Student(c *fiber.Ctx) error {
	var sid string
	if AuthAdmin(c) {
		var data map[string]string

		if err := c.BodyParser(&data); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Failed to parse body",
				"error":   err,
			})
		}

		// Check required fields are included
		if data["sid"] == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"message": "missing required fields",
			})
		}
		sid = data["sid"]
	} else {
		cookie := c.Cookies("jwt")

		token, err := jwt.ParseWithClaims(cookie, &jwt.StandardClaims{}, func(token *jwt.Token) (interface{}, error) {
			return []byte(SecretKey), nil
		})
		// This returns not authorized for both admin and student
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"message": "not authorized",
			})
		}

		claims := token.Claims.(*jwt.StandardClaims)
		sid = claims.Issuer
	}

	responceData := make(map[string]interface{})
	responceData["student"] = nil
	responceData["locker"] = nil
	responceData["contacts"] = nil
	responceData["photo"] = nil

	var student models.Student
	findErr := studentCollection.FindOne(context.TODO(), bson.M{"schooldata.sid": sid}).Decode(&student)
	if findErr != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "student not found",
		})
	}

	responceData["student"] = student

	var locker models.Locker
	if student.SchoolData.Locker != "" {
		lockerCollection.FindOne(context.TODO(), bson.M{"ID": student.SchoolData.Locker}).Decode(&locker)
		responceData["locker"] = locker
	}

	var contacts []models.Contact
	var contact models.Contact
	for i := range student.PersonalData.Contacts {
		findErr := contactCollection.FindOne(context.TODO(), bson.M{"_id": student.PersonalData.Contacts[i]}).Decode(&contact)
		if findErr != nil {
			responceData["error"] = "Error! There was an error finding some contacts"
		}
		contacts = append(contacts, contact)
	}
	if len(contacts) > 0 {
		responceData["contacts"] = contacts
	}

	var photo models.Photo
	findErr = imageCollection.FindOne(context.TODO(), bson.M{"name": student.SchoolData.PhotoName}).Decode(&photo)
	if findErr != nil {
		responceData["error"] = "Error! There was an error finding the student photo"
	}
	responceData["photo"] = photo

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"success":  true,
		"responce": responceData,
	})
}

func Teacher(c *fiber.Ctx) error {
	cookie := c.Cookies("jwt")

	token, err := jwt.ParseWithClaims(cookie, &jwt.StandardClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(SecretKey), nil
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "not authorized",
		})
	}

	claims := token.Claims.(*jwt.StandardClaims)

	var teacher models.Teacher
	findErr := teacherCollection.FindOne(context.TODO(), bson.M{"schooldata.tid": claims.Issuer}).Decode(&teacher)
	if findErr != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "teacher not found",
		})
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"success": true,
		"message": "successfully logged into teacher",
		"result":  teacher,
	})
}

func Admin(c *fiber.Ctx) error {
	cookie := c.Cookies("jwt")

	token, err := jwt.ParseWithClaims(cookie, &jwt.StandardClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(SecretKey), nil
	})
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "not authorized",
		})
	}

	claims := token.Claims.(*jwt.StandardClaims)

	var admin models.Admin
	findErr := adminCollection.FindOne(context.TODO(), bson.M{"aid": claims.Issuer}).Decode(&admin)
	if findErr != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "admin not found",
		})
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"success": true,
		"message": "successfully logged into admin",
		"result":  admin,
	})
}

// Should work for both teacher and student ends
func Logout(c *fiber.Ctx) error {
	cookie := fiber.Cookie{
		Name:     "jwt",
		Value:    "",
		Expires:  time.Now().Add(-time.Hour),
		HTTPOnly: true,
	}
	c.Cookie(&cookie)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "successfully logged out",
	})
}

func CreateContact(c *fiber.Ctx) error {
	var data map[string]interface{}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	if err := c.BodyParser(&data); err != nil {
		cancel()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to parse body",
			"error":   err,
		})
	}

	// Ensure Authenticated admin sent request
	if !AuthAdmin(c) {
		cancel()
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "Unauthorized: only an admin can perform this action",
		})
	}

	// Check required fields are included
	if data["sid"] == nil || data["firstname"] == nil || data["lastname"] == nil || data["homephone"] == nil || data["email"] == nil || data["priority"] == nil || data["relation"] == nil {
		cancel()
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "missing required fields",
		})
	}

	var contact models.Contact
	contact.FirstName = data["firstname"].(string)
	contact.MiddleName = data["middlename"].(string)
	contact.LastName = data["lastname"].(string)
	contact.HomePhone = data["homephone"].(float64)
	contact.WorkPhone = data["workphone"].(float64)
	contact.Email = data["email"].(string)
	contact.Province = data["province"].(string)
	contact.City = data["city"].(string)
	contact.Address = data["address"].(string)
	contact.Postal = data["postal"].(string)
	contact.Relation = data["relation"].(string)
	contact.Priotrity, _ = data["priority"].(float64)

	contact.Created_at, _ = time.Parse(time.RFC3339, time.Now().Format(time.RFC3339))
	contact.Updated_at, _ = time.Parse(time.RFC3339, time.Now().Format(time.RFC3339))
	contact.ID = primitive.NewObjectID()

	_, insertErr := contactCollection.InsertOne(ctx, contact)
	if insertErr != nil {
		cancel()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "could not insert contact",
			"error":   insertErr,
		})
	}

	update := bson.M{
		"$push": bson.M{
			"contacts": contact.ID,
		},
	}
	_, updateErr := studentCollection.UpdateOne(
		ctx,
		bson.M{"sid": data["sid"]},
		update,
	)
	if updateErr != nil {
		cancel()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "the student could not be updated",
			"error":   updateErr,
		})
	}
	defer cancel()

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "successfully inserted contact to student",
	})
}

func DeleteContact(c *fiber.Ctx) error {
	var data map[string]string
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	if err := c.BodyParser(&data); err != nil {
		cancel()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to parse body",
			"error":   err,
		})
	}

	// Ensure Authenticated admin sent request
	if !AuthAdmin(c) {
		cancel()
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "Unauthorized: only an admin can perform this action",
		})
	}

	// Check required fields are included
	if data["id"] == "" {
		cancel()
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "missing required fields",
		})
	}

	_, err := contactCollection.DeleteOne(ctx, bson.M{"_id": data["id"]})
	if err != nil {
		cancel()
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "Failed to delete object",
			"error":   err,
		})
	}
	defer cancel()

	return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
		"success": true,
		"message": "Successfully deleted contact",
	})
}
