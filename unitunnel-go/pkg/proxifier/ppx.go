package proxifier

import (
	"fmt"
	"strings"
)

// GenerateProfile produces an XML configuration profile for Proxifier 3.x and 4.x (.ppx).
// It sets up UniTunnel SOCKS5 for university domains and Psiphon SOCKS5 for general internet,
// while enforcing critical loop-prevention rules to avoid recursive connection crashes.
func GenerateProfile(unitunnelSocksPort, psiphonSocksPort int) string {
	if unitunnelSocksPort <= 0 {
		unitunnelSocksPort = 1080
	}
	if psiphonSocksPort <= 0 {
		psiphonSocksPort = 10808
	}

	var sb strings.Builder
	sb.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"yes\"?>\n")
	sb.WriteString("<ProxifierProfile version=\"101\" platform=\"Windows\" product_id=\"0\" product_minver=\"310\">\n")
	sb.WriteString("  <Options>\n")
	sb.WriteString("    <AutoModeDetection enabled=\"true\"/>\n")
	sb.WriteString("    <ViaProxy enabled=\"false\"/>\n")
	sb.WriteString("    <TryLocalDnsFirst enabled=\"false\"/>\n")
	sb.WriteString("  </Options>\n")
	sb.WriteString("  <ProxyList>\n")
	sb.WriteString(fmt.Sprintf(`    <Proxy id="100" type="SOCKS5">
      <Address>127.0.0.1</Address>
      <Port>%d</Port>
      <Options>48</Options>
    </Proxy>
`, unitunnelSocksPort))
	sb.WriteString(fmt.Sprintf(`    <Proxy id="101" type="SOCKS5">
      <Address>127.0.0.1</Address>
      <Port>%d</Port>
      <Options>48</Options>
    </Proxy>
`, psiphonSocksPort))
	sb.WriteString("  </ProxyList>\n")
	sb.WriteString("  <ChainList/>\n")
	sb.WriteString("  <RuleList>\n")
	sb.WriteString(`    <Rule enabled="true">
      <Name>Prevencion de Bucles (UniTunnel y Psiphon)</Name>
      <Applications>unitunnel.exe; tun2socks.exe; psiphon3.exe; psiphon.exe</Applications>
      <Targets>10.0.0.1; 127.0.0.1; localhost</Targets>
      <Action type="Direct"/>
    </Rule>
`)
	sb.WriteString(`    <Rule enabled="true">
      <Name>Servicios e Intranet UCI</Name>
      <Targets>*.uci.cu; 10.*; 172.16.*; 192.168.*</Targets>
      <Action type="Proxy">100</Action>
    </Rule>
`)
	sb.WriteString(`    <Rule enabled="true">
      <Name>Internet General (Psiphon)</Name>
      <Action type="Proxy">101</Action>
    </Rule>
`)
	sb.WriteString("  </RuleList>\n")
	sb.WriteString("</ProxifierProfile>\n")

	return sb.String()
}
