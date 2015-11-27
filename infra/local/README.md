When you bootstrap a local dev environment, the bootstrapping script will place a local kubeconfig here.
That way, you can use kubectl --kubeconfig for local dev, in the same way you would on dev or prod.
But, that kubeconfig is gitignored, because the IP can be different for each developer.

