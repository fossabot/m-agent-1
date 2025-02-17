// Copyright 2022 LitmusChaos Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package experiment

import (
	"log"
	"net/http"

	errorcodes "github.com/litmuschaos/m-agent/internal/m-agent/errorcodes"
	logger "github.com/litmuschaos/m-agent/internal/m-agent/log"
	"github.com/litmuschaos/m-agent/internal/m-agent/messages"
	"github.com/litmuschaos/m-agent/internal/m-agent/upgrader"
	"github.com/litmuschaos/m-agent/pkg/probes"
	"github.com/litmuschaos/m-agent/pkg/process"
)

// ProcessKill listens for the client actions and executes them as appropriate
func ProcessKill(w http.ResponseWriter, r *http.Request) {

	// upgrade the connection to a websocket connection
	upgrader := upgrader.GetConnectionUpgrader()

	clientMessageReadLogger := logger.GetClientMessageReadErrorLogger()
	steadyStateCheckErrorLogger := logger.GetSteadyStateCheckErrorLogger()
	executeExperimentErrorLogger := logger.GetExecuteExperimentErrorLogger()
	commandProbeExecutionErrorLogger := logger.GetCommandProbeExecutionErrorLogger()
	invalidActionErrorLogger := logger.GetCommandProbeExecutionErrorLogger()

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to establish connection with client, %v", err)
		return
	}

	for {

		action, payload, err := messages.ListenForClientMessage(conn)
		if err != nil {
			if err := messages.SendMessageToClient(conn, "ERROR", errorcodes.GetClientMessageReadErrorPrefix()+err.Error()); err != nil {

				clientMessageReadLogger.Printf("Error occured while sending error message to client, %v", err)
			}
			conn.Close()
			return
		}

		switch action {

		case "CHECK_STEADY_STATE":
			if err := process.ProcessStateCheck(payload); err != nil {

				if err := messages.SendMessageToClient(conn, "ERROR", errorcodes.GetSteadyStateCheckErrorPrefix()+err.Error()); err != nil {
					steadyStateCheckErrorLogger.Printf("Error occured while sending error message to client, %v", err)
				}

				conn.Close()
				return
			}

			if err := messages.SendMessageToClient(conn, "ACTION_SUCCESSFUL", messages.Message{}); err != nil {

				steadyStateCheckErrorLogger.Printf("Error occured while sending feedback message to client, %v", err)
				conn.Close()
				return
			}

		case "EXECUTE_EXPERIMENT":
			if err := process.KillTargetProcesses(payload); err != nil {
				if err := messages.SendMessageToClient(conn, "ERROR", errorcodes.GetExecuteExperimentErrorPrefix()+err.Error()); err != nil {
					executeExperimentErrorLogger.Printf("Error occured while sending error message to client, %v", err)
				}
				conn.Close()
				return
			}

			if err := messages.SendMessageToClient(conn, "ACTION_SUCCESSFUL", messages.Message{}); err != nil {
				executeExperimentErrorLogger.Printf("Error occured while sending feedback message to client, %v", err)
				conn.Close()
				return
			}

		case "EXECUTE_COMMAND":
			stdout, err := probes.ExecuteCmdProbeCommand(payload)

			if err != nil {
				if err := messages.SendMessageToClient(conn, "ERROR", errorcodes.GetCommandProbeExecutionErrorPrefix()+err.Error()); err != nil {
					commandProbeExecutionErrorLogger.Printf("Error occured while sending error message to client, %v", err)
					conn.Close()
					return
				}
			}

			if err := messages.SendMessageToClient(conn, "ACTION_SUCCESSFUL", stdout); err != nil {
				commandProbeExecutionErrorLogger.Printf("Error occured while sending feedback message to client, %v", err)
				conn.Close()
				return
			}

		default:
			if err := messages.SendMessageToClient(conn, "ERROR", errorcodes.GetInvalidActionErrorPrefix()+"Invalid action: "+action); err != nil {
				invalidActionErrorLogger.Printf("Error occured while sending error message to client, %v", err)
			}
			conn.Close()
			return
		}
	}
}
