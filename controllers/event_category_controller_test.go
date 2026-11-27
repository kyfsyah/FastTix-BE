package controllers

import (
	"net/http/httptest"
	"strings"
	"testing"

	"fasttix-backend/config"
	"github.com/gofiber/fiber/v2"
)

func TestEventCategoryControllers_ValidationAndDBGuards(t *testing.T) {
	savedDB := config.DB
	config.DB = nil
	defer func() { config.DB = savedDB }()

	app := fiber.New()
	app.Get("/event-categories", GetAllEventCategories)
	app.Get("/event-categories/:id", GetEventCategoryByID)
	app.Post("/event-categories", CreateEventCategory)
	app.Put("/event-categories/:id", UpdateEventCategory)
	app.Delete("/event-categories/:id", DeleteEventCategory)

	tests := []struct {
		name         string
		method       string
		path         string
		body         string
		expectedCode int
	}{
		{
			name:         "GetAllEventCategories returns 500 when DB is nil",
			method:       "GET",
			path:         "/event-categories",
			expectedCode: fiber.StatusInternalServerError,
		},
		{
			name:         "GetEventCategoryByID invalid ID returns 400",
			method:       "GET",
			path:         "/event-categories/abc",
			expectedCode: fiber.StatusBadRequest,
		},
		{
			name:         "GetEventCategoryByID zero ID returns 400",
			method:       "GET",
			path:         "/event-categories/0",
			expectedCode: fiber.StatusBadRequest,
		},
		{
			name:         "GetEventCategoryByID returns 500 when DB is nil",
			method:       "GET",
			path:         "/event-categories/1",
			expectedCode: fiber.StatusInternalServerError,
		},
		{
			name:         "CreateEventCategory invalid JSON returns 400",
			method:       "POST",
			path:         "/event-categories",
			body:         "{invalid",
			expectedCode: fiber.StatusBadRequest,
		},
		{
			name:         "CreateEventCategory missing nama_kategori returns 400",
			method:       "POST",
			path:         "/event-categories",
			body:         `{"nama_kategori":"   ","slug":"solo-concert"}`,
			expectedCode: fiber.StatusBadRequest,
		},
		{
			name:         "CreateEventCategory missing slug returns 400",
			method:       "POST",
			path:         "/event-categories",
			body:         `{"nama_kategori":"Solo Concert","slug":""}`,
			expectedCode: fiber.StatusBadRequest,
		},
		{
			name:         "CreateEventCategory nama_kategori > 100 chars returns 400",
			method:       "POST",
			path:         "/event-categories",
			body:         `{"nama_kategori":"` + strings.Repeat("a", 101) + `","slug":"valid-slug"}`,
			expectedCode: fiber.StatusBadRequest,
		},
		{
			name:         "CreateEventCategory returns 500 when DB is nil",
			method:       "POST",
			path:         "/event-categories",
			body:         `{"nama_kategori":"Solo Concert","slug":"solo-concert"}`,
			expectedCode: fiber.StatusInternalServerError,
		},
		{
			name:         "UpdateEventCategory invalid ID returns 400",
			method:       "PUT",
			path:         "/event-categories/abc",
			body:         `{"nama_kategori":"Updated"}`,
			expectedCode: fiber.StatusBadRequest,
		},
		{
			name:         "UpdateEventCategory empty body returns 400",
			method:       "PUT",
			path:         "/event-categories/1",
			body:         `{}`,
			expectedCode: fiber.StatusBadRequest,
		},
		{
			name:         "UpdateEventCategory returns 500 when DB is nil",
			method:       "PUT",
			path:         "/event-categories/1",
			body:         `{"deskripsi":"Deskripsi baru"}`,
			expectedCode: fiber.StatusInternalServerError,
		},
		{
			name:         "DeleteEventCategory invalid ID returns 400",
			method:       "DELETE",
			path:         "/event-categories/0",
			expectedCode: fiber.StatusBadRequest,
		},
		{
			name:         "DeleteEventCategory returns 500 when DB is nil",
			method:       "DELETE",
			path:         "/event-categories/1",
			expectedCode: fiber.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.StatusCode != tc.expectedCode {
				t.Errorf("[%s] status = %d, expected %d", tc.name, resp.StatusCode, tc.expectedCode)
			}
		})
	}
}
