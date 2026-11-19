include $(TOPDIR)/rules.mk

PKG_NAME:=teleport-edge
PKG_VERSION:=18.10.0
PKG_RELEASE:=1

PKG_LICENSE:=AGPL-3.0-or-later
PKG_MAINTAINER:=Teleport Edge Agent

include $(INCLUDE_DIR)/package.mk

define Package/teleport-edge
  SECTION:=net
  CATEGORY:=Network
  TITLE:=Teleport Edge SSH agent
  URL:=https://github.com/gravitational/teleport
  DEPENDS:=
endef

define Package/teleport-edge/description
  Reduced Teleport SSH Node for OpenWrt. Provides interactive shell and
  SFTP/SCP through a reverse tunnel to a Teleport proxy.
endef

define Package/teleport-edge/conffiles
/etc/config/teleport-edge
endef

# Prebuilt static binary is copied from the Teleport build tree.
EDGE_BIN:=$(CURDIR)/../../../build/teleport-edge

define Build/Compile
	true
endef

define Package/teleport-edge/install
	$(INSTALL_DIR) $(1)/usr/sbin
	$(INSTALL_BIN) $(EDGE_BIN) $(1)/usr/sbin/teleport-edge
	$(INSTALL_DIR) $(1)/etc/init.d
	$(INSTALL_BIN) ./files/teleport-edge.init $(1)/etc/init.d/teleport-edge
	$(INSTALL_DIR) $(1)/etc/config
	$(INSTALL_CONF) ./files/teleport-edge.config $(1)/etc/config/teleport-edge
	$(INSTALL_DIR) $(1)/etc/uci-defaults
	$(INSTALL_BIN) ./files/uci-defaults $(1)/etc/uci-defaults/90-teleport-edge
endef

$(eval $(call BuildPackage,teleport-edge))
