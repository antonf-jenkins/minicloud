pipeline {
    agent any

    environment {
        SSH_OPTS = '-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null'
        MINICLOUD_PORT = 1959
    }

    stages {
        stage('Build') {
            agent {
                docker { image 'microcloud-ci-build' }
            }

            steps {
                checkout scm

                sh 'mkdir -p vendor'
                dir('vendor') {
                    git '/var/lib/jenkins/minicloud-vendor.git'
                }

                sh 'mkdir -p buildroot/src/github.com/antonf; ln -s "$(pwd)" "$(pwd)/buildroot/src/github.com/antonf/minicloud"'
                sh 'GOPATH="$(pwd)/buildroot" go build github.com/antonf/minicloud/cli/minicloud'
                archive 'minicloud'
                stash name: 'binary', includes: 'minicloud'
            }
        }

        stage('Deploy Test Env') {
            steps {
                unstash 'binary'
                sh 'ls -la'

                script {
                    def ipAddress = sh(returnStdout: true, script: 'sudo minicloud-ci-vm start "${BUILD_TAG}"').trim()
                    env.MINICLOUD_IP = ipAddress
                    echo "Minicloud IP: ${ipAddress}"
                }
                sshagent(credentials: ['minicloud-ci-vm-key']) {
                    // TODO: incorporate this logic into minicloud-ci-vm script
                    timeout(time: 5, unit: 'MINUTES') {
                        retry(100500) {
                            sh 'ssh ${SSH_OPTS} root@${MINICLOUD_IP} /root/wait_ready.sh'
                        }
                    }
                    sh '''
                        scp ${SSH_OPTS} ./minicloud root@${MINICLOUD_IP}:/usr/local/bin/
                        ssh ${SSH_OPTS} root@${MINICLOUD_IP} \
                            "/usr/local/bin/minicloud > /var/log/minicloud.log 2>&1 &"
                    '''
                }
                // Wait for minicloud to start
                timeout(time: 1, unit: 'MINUTES') {
                    sh '''
                        while true; do
                            CODE=$(curl -s -o /dev/null -w "%{http_code}" \
                                "http://${MINICLOUD_IP}:${MINICLOUD_PORT}/projects" || true)
                            if [ "${CODE}" = "200" ]; then
                                break
                            fi
                            sleep 1
                        done
                    '''
                }
            }
        }

        stage('Functional Tests') {
            agent {
                docker {
                    image 'microcloud-ci-test'
                    reuseNode true
                }
            }
            steps {
                timeout(time: 15, unit: 'MINUTES') {
                    sh 'nosetests --with-xunit tests/'
                }
            }
        }
    }

    post {
        always {
            junit '**/nosetests.xml'
            sshagent(credentials: ['minicloud-ci-vm-key']) {
                // TODO: write script that will handle log copying
                sh 'scp ${SSH_OPTS} root@${MINICLOUD_IP}:/var/log/minicloud.log . || touch minicloud.log'
            }
            sh 'sudo minicloud-ci-vm stop "${BUILD_TAG}"'
            archive 'minicloud.log'
        }
    }
}
