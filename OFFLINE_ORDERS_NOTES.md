# Offline Orders 2.2.1

Android Orders is offline-first after the phone has been paired once.

- Orders are saved immediately in encrypted phone storage.
- Last-synced company, buyer, stock-item, unit and buyer-detail data remains available offline.
- Buyer and product names can also be typed when a cached master is unavailable.
- Pending orders can be created, edited and shared as Details only while the PC is unavailable.
- Dispatch can be marked offline and syncs later.
- Dispatched orders remain view-only and expire after 7 days.
- The widget reads the phone cache first and attempts PC sync in the background.
- Offline creates carry an internal phone sync token so a connection drop during create cannot cause a duplicate on retry.
- Android instrumentation verifies behavior against an intentionally unreachable PC and captures Orders + Details only screenshots.
