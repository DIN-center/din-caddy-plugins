package watcher

import (
	"fmt"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

// Initialize the validator when the package is loaded
func init() {
	validate = validator.New()

	// Register custom validation for interval format
	validate.RegisterValidation("interval_format", func(fl validator.FieldLevel) bool {
		interval := fl.Field().String()
		if interval == "" {
			return true // empty values are allowed
		}
		// Match {number}[min|h|d|w]
		re := regexp.MustCompile(`^\d+(min|h|d|w)$`)
		return re.MatchString(interval)
	})
}

func validateQueryParams(params interface{}) error {
	err := validate.Struct(params)
	if err != nil {
		// Format validation errors into a human-readable format
		var validationErrors []string
		for _, err := range err.(validator.ValidationErrors) {
			validationErrors = append(validationErrors, fmt.Sprintf("Field '%s': %s", err.Field(), err.Tag()))
		}
		return fmt.Errorf("validation failed: %s", strings.Join(validationErrors, ", "))
	}
	return nil
}

// buildQuery builds a query string from the given parameters.
func buildQuery(params interface{}) string {
	values := url.Values{}

	// Reflect on the struct fields
	v := reflect.ValueOf(params)
	t := reflect.TypeOf(params)

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)
		queryKey := fieldType.Tag.Get("query")

		// Skip empty fields
		if queryKey == "" || (field.Kind() == reflect.String && field.String() == "") {
			continue
		}

		switch field.Kind() {
		case reflect.String:
			values.Add(queryKey, field.String())
		case reflect.Slice:
			slice := field.Interface().([]string)
			if len(slice) > 0 {
				values.Add(queryKey, strings.Join(slice, ","))
			}
		case reflect.Int:
			if field.Int() > 0 {
				values.Add(queryKey, strconv.Itoa(int(field.Int())))
			}
		}
	}

	return values.Encode()
}
