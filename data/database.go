package data

import (
	"fmt"
	"time"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"

	"golang.org/x/crypto/bcrypt"
)

type Database interface {
	Close() error
	GetTask(taskId string, userId uint64, kind *TaskKind) (*Task, error)
	GetTasks(userId uint64, kind *TaskKind) ([]Task, error)
	AddTask(task *Task, userId uint64) error
	DeleteTask(taskId string, userId uint64) (bool, error)
	UpdateTask(taskId string, userId uint64, attrs map[string]interface{}) (*Task, error)
	CreateUser(username string, password string) (*User, error)
	GetUserById(id uint64) (*User, error)
	GetUserByUsername(username string) (*User, error)
	AddAction(action *Action, userId uint64) error
	DeleteAction(id string, userId uint64) error
}

type gormDB struct {
	*gorm.DB
}

type TaskKind int

const (
	TaskEnum TaskKind = iota
	HabitEnum
)

type Interval int

const (
	Daily Interval = iota
	Weekly
	Monthly
)

type Task struct {
	// Common fields
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	DeletedAt *time.Time
	Id        string   `json:"id" gorm:"primary_key;type:uuid;default:uuid_generate_v4()"`
	Kind      TaskKind `json:"kind" gorm:"not_null"`
	Title     string   `json:"title" gorm:"not_null"`
	Done      bool     `json:"done" gorm:"not_null;default:false"`
	UserId    uint64   `json:"user_id" gorm:"not_null"`
	Actions   []Action `json:"actions" gorm:"ForeignKey:TaskId"`
	// Task Fields
	StartDate *time.Time `json:"start_date"`
	EndDate   *time.Time `json:"end_date"`
	// Habit Fields
	Interval  Interval `json:"interval"`
	Frequency int      `json:"frequency"`
}

type ActionKind int

const (
	ActionProgress ActionKind = iota
	ActionDefer
	ActionDone
)

type Action struct {
	Id     string     `json:"id" gorm:"primary_key;type:uuid;default:uuid_generate_v4()"`
	Kind   ActionKind `json:"kind" gorm:"not_null"`
	When   *time.Time `json:"when" gorm:"not_null"`
	TaskId string     `json:"task_id" gorm:"not_null;type:uuid"`
}

type User struct {
	Id             uint64 `json:"id" gorm:"primary_key"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
	Username       string `json:"username" gorm:"not_null;unique"`
	HashedPassword []byte `json:"-" gorm:"not_null"`
	Tasks          []Task `json:"-" gorm:"ForeignKey:UserId"`
}

func InitDatabase(dialect string, host string, user string, dbName string) Database {
	db, err := gorm.Open(dialect, fmt.Sprintf("host=%s user=%s DB.name=%s sslmode=disable", host, user, dbName))
	if err != nil {
		panic(err)
	}
	db.AutoMigrate(&Task{}, &User{}, &Action{})
	return gormDB{db}
}

func (db gormDB) Close() error {
	return db.Close()
}

func (db gormDB) GetTask(taskId string, userId uint64, kind *TaskKind) (*Task, error) {
	whereFields := map[string]interface{}{
		"id":      taskId,
		"user_id": userId,
	}
	if kind != nil {
		whereFields["kind"] = *kind
	}

	var task Task
	// TODO: Only preload actions if necessary
	if err := db.Preload("Actions").Where(whereFields).First(&task).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

func (db gormDB) GetTasks(userId uint64, kind *TaskKind) ([]Task, error) {
	whereFields := map[string]interface{}{
		"user_id": userId,
	}
	if kind != nil {
		whereFields["kind"] = *kind
	}

	var tasks []Task
	// TODO: Only preload actions if necessary
	if err := db.Preload("Actions").Where(whereFields).Find(&tasks).Error; err != nil {
		return nil, err
	}
	return tasks, nil
}

func (db gormDB) AddTask(task *Task, userId uint64) error {
	task.UserId = userId
	return db.Create(task).Error
}

// Deletes the task with the given ID and returns whether a row was deleted.
func (db gormDB) DeleteTask(taskId string, userId uint64) (bool, error) {
	task := Task{
		Id:     taskId,
		UserId: userId,
	}
	result := db.Where(&task).Delete(&task)
	if err := result.Error; err != nil {
		return false, err
	}
	return result.RowsAffected > 0, nil
}

// Updates a task with the given attributes and returns the updated Task if one exists for the ID.
func (db gormDB) UpdateTask(taskId string, userId uint64, attrs map[string]interface{}) (*Task, error) {
	task := Task{
		Id: taskId,
	}
	result := db.Model(&task).Where("user_id = ?", userId).Updates(attrs)
	if err := result.Error; err != nil {
		return nil, err
	}
	if result.RowsAffected == 0 {
		return nil, fmt.Errorf("Task ID \"%s\" does not exist for user \"%d\"", taskId, userId)
	}
	// TODO: Only query actions if necessary
	if err := db.Model(&task).Related(&task.Actions).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

func (db gormDB) CreateUser(username string, password string) (*User, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return nil, err
	}

	user := &User{
		Username:       username,
		HashedPassword: hashedPassword,
	}

	err = db.Create(user).Error
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (db gormDB) GetUserById(id uint64) (*User, error) {
	user := &User{
		Id: id,
	}
	if err := db.Where(user).First(user).Error; err != nil {
		return nil, err
	}
	return user, nil
}

func (db gormDB) GetUserByUsername(username string) (*User, error) {
	user := &User{
		Username: username,
	}
	if err := db.Where(user).First(user).Error; err != nil {
		return nil, err
	}
	return user, nil
}

func (db gormDB) AddAction(action *Action, userId uint64) error {
	task, err := db.GetTask(action.TaskId, userId, nil)
	if task == nil {
		return fmt.Errorf("Task %s does not exist for user %d", action.TaskId, userId)
	}
	if err != nil {
		return err
	}
	return db.Create(action).Error
}

func (db gormDB) DeleteAction(id string, userId uint64) error {
	action := &Action{
		Id: id,
	}
	if err := db.Where(action).First(action).Error; err != nil {
		return err
	}
	task, err := db.GetTask(action.TaskId, userId, nil)
	if err != nil {
		return err
	}
	if task == nil {
		return fmt.Errorf("Not authorized to delete action %s", id)
	}
	return db.Delete(action).Error
}
