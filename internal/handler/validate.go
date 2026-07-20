package handler

import (
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"

	"github.com/kalia/store/pkg/apperr"
)

var validate = validator.New(validator.WithRequiredStructEnabled())

// bindAndValidate parses the JSON body into dst and runs struct-tag validation.
func bindAndValidate(c *fiber.Ctx, dst any) error {
	if err := c.BodyParser(dst); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	if err := validate.Struct(dst); err != nil {
		return apperr.BadRequest(formatValidationError(err))
	}
	return nil
}

func formatValidationError(err error) string {
	var verrs validator.ValidationErrors
	if !asValidationErrors(err, &verrs) {
		return "validation failed"
	}
	msgs := make([]string, 0, len(verrs))
	for _, fe := range verrs {
		msgs = append(msgs, fe.Field()+" failed "+fe.Tag()+" validation")
	}
	return strings.Join(msgs, "; ")
}

func asValidationErrors(err error, target *validator.ValidationErrors) bool {
	if verrs, ok := err.(validator.ValidationErrors); ok {
		*target = verrs
		return true
	}
	return false
}
