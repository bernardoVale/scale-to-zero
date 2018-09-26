

# Scenarios

# Happy Path

1. Request sent, backend returns 503
1. default-backend check redis key with status `sleeping`
1. default-backend publish `wakeup` message
1. downscaler wakeup process receives the notification
1. downscaler tries to scale the app
1. downscaler writes new status `waking_up`

---

# Waking Up

1. Request sent, backend returns 503
1. default-backend checks redis key with status `waking_up`
1. default-backend returns sleeping content to the client
1. At some point the app starts

---

# Manual scale down

1. Request sent, backend returns 503
1. default-backend checks redis key with status `waking_up`
1. default-backend returns sleeping content to the client

---

# No key

1. Request sent, backend returns 503
1. default-backend checks redis key. Key doesn't exists
1. default-backend returns 404

---

