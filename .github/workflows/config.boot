system {
    host-name vyos
    login {
        user vyos {
            authentication {
                plaintext-password "vyos"
            }
        }
    }
    syslog {
        global {
            facility all {
                level info
            }
            facility protocols {
                level debug
            }
        }
    }
    ntp {
        server "time1.vyos.net"
        server "time2.vyos.net"
        server "time3.vyos.net"
    }
    console {
        device ttyS0 {
            speed 115200
        }
    }
    config-management {
        commit-revisions 100
    }
}
interfaces {
    loopback lo {
    }
}
service {
    https {
        api {
            keys {
                id apikey {
                    key vyos
                }
            }
        }
        api-restrict {
            virtual-host vyos
        }
        virtual-host vyos {
            listen-address 0.0.0.0
            listen-port 443
            server-name vyos.local
        }
    }
}
