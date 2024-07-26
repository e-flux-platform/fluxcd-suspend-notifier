# fluxcd-suspend-notifier

Application that watches for suspend status changes of fluxcd resources. This is designed to operate in the context of 
fluxcd running in a GKE cluster.

- GKE audit logs are tailed to observe when fluxcd resources are mutated. We use this mechanism specifically as it  
  contains details of the user that has made the modification
- The kubernetes API is used to check if the suspend status has changed (resource modifications can occur for other
  reasons)
- If the suspend status has changed, a notification is dispatched via Slack
