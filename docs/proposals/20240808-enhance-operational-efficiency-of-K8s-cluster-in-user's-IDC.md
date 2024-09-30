# 黄晨钊 (huangchenzhao)-245fc0132-final report

## Project Information
- Project Name：

  Enhance operational efficiency of K8s cluster in user's IDC

- Scheme Description：
  
  For K8s clutesrs in user's IDC, it is difficult to operate, manage and upgrade the control plane components. Users typically adopt the following three solutions to manage K8s clusters in their IDC. 

  - Some users only set up a single K8s cluster in IDC for tenant. In this case, when K8s have version upgrades and changes, about three major releases per year, users will suffer from complex operations to upgrade those components. Meanwhile, there is no resource elasticity capability in K8s clutesrs in user's IDC, such as scaling control plane components, which is a costly operation for user.

  - Some users adopt the KOK architecture in their own IDC to manages tenant-K8s's control plane components. Both host-K8s and tenant-K8s are in user's IDC. In this case, operating and updating control plane components of tenant-K8s will be easy, however, it is still hard to operate and upgrade the control plane components in host-K8s.

  - More and more users only access their IDC machines to cloud service providers as worker nodes, utilizing the abilities of cloud-edge collaboration provided by OpenYurt. But there are some users needs continuous deployment for offline tasks, depending on strong stability of cloud-edge communication, in this case, they tend to maintain a K8s cluster in their IDC.

  This project solves the pain points mentioned above, which automates the operation and maintenance of control plane components of tenant-K8s to replace manual user operations, and affords users who needs continuous deployment for offline tasks a efficient operation scheme to manage their IDC K8s cluster.

- Time Planning：
  - 2024.07: Thorough discussion with mentor to determine the direction and design of the topic.
  - 2024.08: Submitting design documents and reading openyurt source code.
  - 2024.09: Functional code development, and corresponding unit and e2e testing.


## Project Schedule

- The Accomplished Work ：

  - Reduce the complexity of management and operation, and improve operational efficiency for users.

  - Optimize the architecture of the IDC K8s cluster to enhance stability, reliability and security.
    1. KCM (kube-controller-manager) and Scheduler are deployed as `deployment`, ETCD is deployed as `statefulset`. KubeAPIServer is deployed as `daemonset` on worker nodes of host-K8s, once a new machine are accessed to host-K8s, KubeAPIServer will be autoscaling.
    2. KCM and Scheduler access KubeAPIServer by the service of KubeAPIServer, KubeAPIServer access ETCD by the service of ETCD. All the service are support in host-K8s, so there is no need to introduce CoreDNS.
    3. Worker nodes of tenant-K8s implement load balancing access to KubeAPIServer, dynamically sensing the changes of KubeAPIServer, so there is no need to introduce loadbalancer in front of KubeAPIServer.
    4. Business `pod` and control plane components are naturally separated by deploying control plane components in form of `pod` in host-K8s, which affords higher security for users.

  In tenant-K8s, the designed details are as follows:
  - In control plane nodepool, we will afford users the template of control plane components:
    1. KCM and Scheduler are deployed as `deployment`, which both have two replicas. KCM and Scheduler access KubeAPIServer by it's service.
    2. KubeAPIServer is deployed as `daemonset`. KubeAPIServer access ETCD by service of ETCD.
    3. There are two type of ETCD: data and event, which are both deployed as `statefulset`.

  - In local nodepool:
    1. We add a new `local` mode in YurtHub. In `local` mode, YurtHub will maintain a loadbalance rule, allowing components like Kubelet to load balancing access to the KubeAPIServer.
    2. yurtadm join affords users to access nodes in their own IDCs to tenant-K8s in `local` mode.

  `local` mode YurtHub gets pod's ip in host-K8s's apiserver, and maintains the loadbalance rule to afford load balancing access to APIServer-pods.

- Problem and Solution：
  - Problem: The yurthub source code is cumbersome, and a reasonable architecture needs to be designed to be compatible with the original code.
  
  - Solution: Fully read the source code and discussions with the instructor to figure out the program design.


- Subsequent Work Arrangement：

  - In future, we plan to afford the admin node for users to use tools like kubectl to access and operate tenant-K8s.