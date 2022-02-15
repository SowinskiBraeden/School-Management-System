package routes

import (
	"github.com/SowinskiBraeden/school-management-api/controllers"

	"github.com/gofiber/fiber/v2"
)

func Setup(app *fiber.App) {
	// API Handling

	var routerPrefix string = "/api/v1"

	// Student Authentication Handler
	app.Get(routerPrefix+"/student", controllers.Student)
	app.Post(routerPrefix+"/student/enroll", controllers.Enroll)
	app.Post(routerPrefix+"/student/login", controllers.StudentLogin)

	// Update Student Handler
	app.Post(routerPrefix+"/student/updateName", controllers.UpdateStudentName)
	app.Post(routerPrefix+"/student/updateGradeLevel", controllers.UpdateStudentGradeLevel)
	app.Post(routerPrefix+"/student/updateHomeroom", controllers.UpdateStudentHomeroom)
	app.Post(routerPrefix+"/student/updateLocker", controllers.UpdateStudentLocker)
	app.Post(routerPrefix+"/studnet/updateYOG", controllers.UpdateStudentYOG)
	app.Post(routerPrefix+"/studnet/addContact", controllers.AddStudentContact)
	app.Post(routerPrefix+"/student/removeContact", controllers.RemoveStudentContact)
	app.Post(routerPrefix+"/student/updatePassword", controllers.UpdateStudentPassword)
	app.Post(routerPrefix+"/student/resetPassword", controllers.ResetStudentPassword)
	app.Post(routerPrefix+"/student/updateAddress", controllers.UpdateStudentAddress)
	app.Post(routerPrefix+"/student/updatePhoto", controllers.UpdateStudentPhoto)
	app.Post(routerPrefix+"/student/updateEmail", controllers.UpdateStudentEmail)

	// Student Contact Handler
	app.Post(routerPrefix+"/student/createContact", controllers.CreateContact)
	app.Post(routerPrefix+"/student/contact/updateName", controllers.UpdateContactName)
	app.Post(routerPrefix+"/student/contact/updateAddress", controllers.UpdateContactAddress)
	app.Post(routerPrefix+"/student/contact/updateHomePhone", controllers.UpdateContactHomePhone)
	app.Post(routerPrefix+"/student/contact/updateWorkPhone", controllers.UpdateContactWorkPhone)
	app.Post(routerPrefix+"/student/contact/updateEmail", controllers.UpdateContactEmail)
	app.Post(routerPrefix+"/student/contact/updatePriority", controllers.UpdateContactPriority)
	app.Post(routerPrefix+"/student/contact/deleteContact", controllers.DeleteContact)

	// Teacher Authentication Handler
	app.Get(routerPrefix+"/teacher", controllers.Teacher)
	app.Post(routerPrefix+"/teacher/register", controllers.RegisterTeacher)
	app.Post(routerPrefix+"/teacher/login", controllers.TeacherLogin)

	// Teacher Update Handler
	app.Post(routerPrefix+"/teacher/updatePassword", controllers.UpdateTeacherPassword)
	app.Post(routerPrefix+"/teacher/updateAddress", controllers.UpdateTeacherAddress)
	app.Post(routerPrefix+"/teacher/updatePhoto", controllers.UpdateTeacherPhoto)
	app.Post(routerPrefix+"/teacher/updateName", controllers.UpdateTeacherName)
	app.Post(routerPrefix+"/teacher/updateHomeroom", controllers.UpdateTeacherHomeroom)
	app.Post(routerPrefix+"/teacher/updateEmail", controllers.UpdateTeacherEmail)
	app.Post(routerPrefix+"/teacher/resetPassword", controllers.ResetTeacherPassword)

	// General Routes
	app.Post(routerPrefix+"/logout", controllers.Logout)

	// Admin Login Handling
	app.Get(routerPrefix+"/admin", controllers.Admin)
	app.Post(routerPrefix+"/admin/create", controllers.CreateAdmin)
	app.Post(routerPrefix+"/admin/login", controllers.AdminLogin)

	// General Command Handling
	app.Post(routerPrefix+"/admin/updateLockerCombo", controllers.UpdateLockerCombo)
	app.Post(routerPrefix+"/admin/renableStudent", controllers.RemoveStudentsDisabled)
	app.Post(routerPrefix+"/admin/renableTeacher", controllers.RemoveTeachersDisabled)
}
