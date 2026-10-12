package main

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/rodroguett/ai-doc-platform/internal/gateway"
	"github.com/rodroguett/ai-doc-platform/internal/platform/cloudtasks"
	"github.com/rodroguett/ai-doc-platform/internal/platform/config"
)

// tasksDispatcher despacha un trabajo pidiéndole a Cloud Tasks que invoque
// la operación que lo ejecuta, en este mismo servicio.
//
// Vive en el binario y no en un módulo porque une a dos: sabe qué ruta del
// gateway ejecuta un trabajo de rag, algo que ninguno de los dos debe saber
// del otro.
type tasksDispatcher struct {
	tasks      *cloudtasks.Client
	serviceURL string
	token      string
}

func newTasksDispatcher(cfg config.Config) (*tasksDispatcher, error) {
	serviceURL := strings.TrimRight(cfg.ServiceURL, "/")
	if !strings.HasPrefix(serviceURL, "https://") {
		return nil, errors.New("SERVICE_URL debe ser la dirección https del servicio cuando se usa CLOUD_TASKS_QUEUE")
	}
	// La tarea se autoriza con el token de escritura: sin él, la operación
	// que ejecuta el trabajo la rechazaría.
	if cfg.WriteToken == "" {
		return nil, errors.New("API_WRITE_TOKEN es necesaria cuando se usa CLOUD_TASKS_QUEUE")
	}
	tasks, err := cloudtasks.New(cfg.TasksQueue)
	if err != nil {
		return nil, err
	}
	return &tasksDispatcher{tasks: tasks, serviceURL: serviceURL, token: cfg.WriteToken}, nil
}

func (d *tasksDispatcher) Enqueue(ctx context.Context, jobID uuid.UUID) error {
	return d.tasks.Enqueue(ctx, cloudtasks.Task{
		URL:     d.serviceURL + gateway.RunJobPath(jobID),
		Headers: map[string]string{"Authorization": "Bearer " + d.token},
	})
}
