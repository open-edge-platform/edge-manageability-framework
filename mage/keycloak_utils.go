package mage

// https://www.keycloak.org/server/bootstrap-admin-recovery
import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/magefile/mage/mg"
)

var keycloak_namespace = "orch-platform"
var char_length = 10

type Keycloak mg.Namespace

func (k Keycloak) GetPassword() {
	command := "kubectl get secret -n " + keycloak_namespace + " platform-keycloak -o jsonpath='{.data.admin-password}' | base64 --decode"
	out, err := exec.Command("bash", "-c", command).CombinedOutput()
	if err != nil {
		fmt.Println("Error executing command:", err)
		return
	}
	fmt.Println(string(out))
}

// set_password '<password>' sets the keycloak password, make sure you use quotes around the password
func (k Keycloak) SetPassword(password string) {
	encoded_password := check_and_encode_password(password)
	if encoded_password == "" {
		fmt.Println("Password does not meet the requirements.")
		return
	}
	set_keycloak_password(encoded_password)
	k.ResetPassword()
}

// Resets the keycloak password and restarts keycloak
func (k Keycloak) ResetPassword() {

	_, decodedMap := get_postgress_creds()
	start_local_psql_pod()
	admin_id := run_sql_command(decodedMap, "SELECT id FROM user_entity where username = '\\'admin\\'';")
	run_sql_command(decodedMap, "DELETE from user_role_mapping where user_id = '\\'"+admin_id+"\\'';")
	run_sql_command(decodedMap, "DELETE from credential where user_id = '\\'"+admin_id+"\\'';")
	run_sql_command(decodedMap, "DELETE from user_attribute where user_id = '\\'"+admin_id+"\\'';")
	run_sql_command(decodedMap, "DELETE FROM user_required_action where user_id = '\\'"+admin_id+"\\'';")
	run_sql_command(decodedMap, "DELETE from user_entity where id = '\\'"+admin_id+"\\'';")

	run_keycloak_admin_bootstrap()

	clean_up_psql_pod()
	fmt.Println("Keycloak password reset complete.")
}

func check_and_encode_password(password string) string {
	// Check password length
	if len(password) < char_length {
		fmt.Println("Password must be at least 10 characters long")
		return ""
	}
	// Check for at least one special character
	specialCharRegex := regexp.MustCompile(`[!@#$%^&*(),.?":{}|<>]`)
	if !specialCharRegex.MatchString(password) {
		fmt.Println("Password must contain at least one special character")
		return ""
	}

	// Check for at least one digit
	digitRegex := regexp.MustCompile(`[0-9]`)
	if !digitRegex.MatchString(password) {
		fmt.Println("Password must contain at least one digit")
		return ""
	}

	// Check for at least one uppercase letter
	uppercaseRegex := regexp.MustCompile(`[A-Z]`)
	if !uppercaseRegex.MatchString(password) {
		fmt.Println("Password must contain at least one uppercase letter")
		return ""
	}
	return base64.StdEncoding.EncodeToString([]byte(password))
}

func clean_up_psql_pod() {
	command := "kubectl delete -n " + keycloak_namespace + " pod/keycloak-recovery-psql"
	exec.Command("bash", "-c", command).CombinedOutput()
}

func set_keycloak_password(encoded_password string) {
	command := "kubectl -n " + keycloak_namespace + " get secret platform-keycloak -o yaml | yq e '.data.admin-password = \"" + encoded_password + "\"' | kubectl apply --force -f -"
	exec.Command("bash", "-c", command).CombinedOutput()
	command = "kubectl delete pod -n " + keycloak_namespace + " platform-keycloak-0"
	exec.Command("bash", "-c", command).CombinedOutput()
	fmt.Printf("Waiting for keycloak to restart...")
	time.Sleep(45 * time.Second)
}

func start_local_psql_pod() {
	command := "kubectl run -n " + keycloak_namespace + " keycloak-recovery-psql --image=bitnami/postgresql -- sh -c 'sleep 10000'"
	_, err := exec.Command("bash", "-c", command).CombinedOutput()
	if err != nil {
		fmt.Println(err.Error())
	}
	fmt.Println("Spinning up an pod in the same namespace as keycloak.")
	command = "kubectl logs -n " + keycloak_namespace + " keycloak-recovery-psql"

	for {
		outputlog, err := exec.Command("bash", "-c", command).CombinedOutput()
		if err != nil {
			continue // If there's an error, continue to the next iteration
		}

		output := strings.TrimSpace(string(outputlog)) // Trim any whitespace or newline characters

		if strings.Contains(output, "Welcome to the Bitnami postgresql container") {
			break // Exit the loop if the output matches the target
		}
	}
}

func run_keycloak_admin_bootstrap() {
	// some strange encoding issue with the keycloak shell requires the export.
	command := "kubectl exec -itn " + keycloak_namespace + " platform-keycloak-0" +
		"  -- sh -c 'export KC_BOOTSTRAP_ADMIN_PASSWORD=\"$(echo $KC_BOOTSTRAP_ADMIN_PASSWORD)\"; /opt/bitnami/keycloak/bin/kc.sh bootstrap-admin user --username:env KC_BOOTSTRAP_ADMIN_USERNAME --password:env KC_BOOTSTRAP_ADMIN_PASSWORD'"
	out, err := exec.Command("bash", "-c", command).CombinedOutput()
	if err != nil {
		fmt.Println(string(out), err)
	}
}

func run_sql_command(decodedMap map[string]string, sqlCommand string) string {
	// sqlCommand := "SELECT id FROM user_entity where username = '\\'admin\\'';" // Example SQL command
	psqlCommand := fmt.Sprintf(
		"PGPASSWORD=%s psql -h %s -p %s -U %s -d %s -t -c \"%s\"",
		decodedMap["PGPASSWORD"],
		decodedMap["PGHOST"],
		decodedMap["PGPORT"],
		decodedMap["PGUSER"],
		decodedMap["PGDATABASE"],
		sqlCommand,
	)

	command := "kubectl exec -i -n " + keycloak_namespace + " pod/keycloak-recovery-psql -- sh -c ' " + psqlCommand + "'"
	sql_output_byte, err := exec.Command("bash", "-c", command).CombinedOutput()
	if err != nil {
		// Print debug information
		fmt.Println("psql command to execute:")
		fmt.Println(string(command))
		fmt.Println("Bash command to execute:")
		fmt.Println(psqlCommand)
		fmt.Println(string(sql_output_byte))
		return "error"
	}
	sql_output := strings.TrimSpace(string(sql_output_byte))
	fmt.Println(sql_output)
	return sql_output
}

func get_postgress_creds() (error, map[string]string) {
	json_project, _ := exec.Command("kubectl", "get", "secret", "-n", keycloak_namespace, "platform-keycloak-local-postgresql", "-o", "json").CombinedOutput()

	// Create a new map to store decoded values
	decodedMap := make(map[string]string)

	var data map[string]interface{}
	if err := json.Unmarshal(json_project, &data); err != nil {
		fmt.Println("Error unmarshalling JSON:", err)
		return err, decodedMap
	}

	encodedMap, ok := data["data"].(map[string]interface{})
	if !ok {
		fmt.Println("Error: data['data'] is not of type map[string]interface{}")
		fmt.Println(data["data"])
		return nil, decodedMap
	}
	// Function to decode base64 strings
	decodeBase64 := func(encoded string) (string, error) {
		decodedBytes, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return err.Error(), nil
		}
		return string(decodedBytes), nil
	}

	// Iterate over the map and print decoded values
	for key, encodedValue := range encodedMap {
		// Convert the value to a string
		valueStr, ok := encodedValue.(string)
		if !ok {
			fmt.Printf("Error: value for key %s is not a string\n", key)
			continue
		}

		decodedValue, err := decodeBase64(valueStr)
		if err != nil {
			fmt.Printf("Error decoding value for key %s: %v\n", key, err)
			continue
		}

		fmt.Printf("%s: %s\n", key, decodedValue)
		// Store the decoded value in the new map
		decodedMap[key] = decodedValue
	}

	// fmt.Println(decodedMap["PGDATABASE"])
	return nil, decodedMap
}
