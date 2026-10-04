# TallyBridge final production release rules.
# AndroidX Credentials and Google Identity ship their own consumer ProGuard rules.
# Preserve generic signatures and annotations used by Android/Google libraries.
-keepattributes Signature,*Annotation*,InnerClasses,EnclosingMethod
