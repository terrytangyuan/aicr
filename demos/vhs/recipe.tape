Require aicr

Set Shell "bash"
Set FontSize 21
Set Width 1400
Set Height 800
Set WindowBar Colorful
Set Margin 10
Set MarginFill "#5c9300"
Set BorderRadius 5

Type "# simple recipe"
Enter
Type "aicr recipe --accelerator h100 --output recipe.yaml"
Sleep 1s
Enter
Sleep 3s
Ctrl+L

Type "# view resulting YAML"
Enter
Type "yq . recipe.yaml | bat -l yaml"
Enter
Sleep 3s
Ctrl+V
Sleep 3s
Ctrl+V
Sleep 3s
Ctrl+V
Ctrl+C
Ctrl+L

Type "# more specific recipe"
Enter
Type "aicr recipe \"
Enter
Type "  --service eks \"
Enter
Type "  --accelerator h100 \"
Enter
Type "  --intent training \"
Enter
Type "  --os ubuntu \"
Enter
Type "  --platform kubeflow \"
Enter
Type "  --output recipe.yaml"
Sleep 1s
Enter
Sleep 3s
Ctrl+L

Type "# recipe from cluster snapshot"
Enter
Type "kubectl get nodes"
Sleep 1s
Enter
Sleep 5s
Ctrl+L

Type "# snapshot cluster"
Enter
Type "aicr snapshot \"
Enter
Type "  --namespace aicr-demo \"
Enter
Type "  --node-selector nodeGroup=gpu-worker \"
Enter
Type "  --toleration dedicated=worker-workload:NoSchedule \"
Enter
Type "  --toleration dedicated=worker-workload:NoExecute \"
Enter
Type "  --output snapshot.yaml"
Sleep 1s
Enter
Sleep 8s
Ctrl+L

Type "# recipe from snapshot"
Enter
Type "aicr recipe \"
Enter
Type "  --snapshot snapshot.yaml \"
Enter
Type "  --intent training \"
Enter
Type "  --output recipe-from-snapshot.yaml"
Sleep 1s
Enter
Sleep 3s
Ctrl+L
