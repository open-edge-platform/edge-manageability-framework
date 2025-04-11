## Mailpit deployment 
Deployment in this directory is used only by `dev` cluster profile, for testing alert emailing feature.
This deployment is not intended for production environment.
Alerting-emails-dev profile is required for receiving test email notifications.
## Files
### mail_catcher.yaml
Mailpit and its service for receiving test email notifications.
### smtp_secret.yaml
Secret required for proper initialization of email notifications feature in alerting-monitor.