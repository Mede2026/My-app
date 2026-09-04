package w32

import (
	"errors"
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Telechargement par WinHTTP, la bibliotheque HTTP integree a Windows.
//
// Passer par le systeme evite d'embarquer la pile TLS de Go, qui pese a elle
// seule pres de deux megaoctets. Les certificats, les redirections et le proxy
// de l'entreprise ou de l'ecole sont geres par Windows, comme pour n'importe
// quel autre logiciel de la machine.

const (
	winhttpAccessTypeAutomaticProxy = 4
	winhttpFlagSecure               = 0x00800000
	winhttpQueryStatusCode          = 19
	winhttpQueryFlagNumber          = 0x20000000
	httpsPort                       = 443

	// Garde-fou : une reponse plus grosse que cela n'est pas une mise a jour.
	maxDownloadBytes = 64 << 20
)

var (
	winhttp = windows.NewLazySystemDLL("winhttp.dll")

	procWinHTTPOpen               = winhttp.NewProc("WinHttpOpen")
	procWinHTTPConnect            = winhttp.NewProc("WinHttpConnect")
	procWinHTTPOpenRequest        = winhttp.NewProc("WinHttpOpenRequest")
	procWinHTTPSendRequest        = winhttp.NewProc("WinHttpSendRequest")
	procWinHTTPReceiveResponse    = winhttp.NewProc("WinHttpReceiveResponse")
	procWinHTTPQueryHeaders       = winhttp.NewProc("WinHttpQueryHeaders")
	procWinHTTPQueryDataAvailable = winhttp.NewProc("WinHttpQueryDataAvailable")
	procWinHTTPReadData           = winhttp.NewProc("WinHttpReadData")
	procWinHTTPCloseHandle        = winhttp.NewProc("WinHttpCloseHandle")
)

// HTTPGet telecharge une adresse https et rend son contenu.
//
// `agent` est le nom que l'application donne d'elle-meme ; l'interface de
// GitHub l'exige. `accept` peut preciser le type de reponse voulu.
func HTTPGet(address, agent, accept string) ([]byte, error) {
	host, path, err := splitHTTPS(address)
	if err != nil {
		return nil, err
	}

	session, _, _ := procWinHTTPOpen.Call(
		uintptr(unsafe.Pointer(Str(agent))), winhttpAccessTypeAutomaticProxy, 0, 0, 0,
	)
	if session == 0 {
		return nil, errors.New("WinHTTP indisponible")
	}
	defer procWinHTTPCloseHandle.Call(session)

	connection, _, _ := procWinHTTPConnect.Call(
		session, uintptr(unsafe.Pointer(Str(host))), httpsPort, 0,
	)
	if connection == 0 {
		return nil, fmt.Errorf("connexion a %s impossible", host)
	}
	defer procWinHTTPCloseHandle.Call(connection)

	request, _, _ := procWinHTTPOpenRequest.Call(
		connection, uintptr(unsafe.Pointer(Str("GET"))), uintptr(unsafe.Pointer(Str(path))),
		0, 0, 0, winhttpFlagSecure,
	)
	if request == 0 {
		return nil, errors.New("requete impossible")
	}
	defer procWinHTTPCloseHandle.Call(request)

	headers := "Accept: " + accept + "\r\n"
	encoded := windows.StringToUTF16(headers)
	if ok, _, _ := procWinHTTPSendRequest.Call(
		request, uintptr(unsafe.Pointer(&encoded[0])), uintptr(len(encoded)-1), 0, 0, 0, 0,
	); ok == 0 {
		return nil, errors.New("envoi de la requete impossible (pas de reseau ?)")
	}
	if ok, _, _ := procWinHTTPReceiveResponse.Call(request, 0); ok == 0 {
		return nil, errors.New("aucune reponse du serveur")
	}

	if status := responseStatus(request); status != 200 {
		return nil, fmt.Errorf("le serveur a repondu %d", status)
	}
	return readAll(request)
}

func responseStatus(request uintptr) uint32 {
	var status uint32
	size := uint32(unsafe.Sizeof(status))
	procWinHTTPQueryHeaders.Call(
		request, winhttpQueryStatusCode|winhttpQueryFlagNumber, 0,
		uintptr(unsafe.Pointer(&status)), uintptr(unsafe.Pointer(&size)), 0,
	)
	return status
}

func readAll(request uintptr) ([]byte, error) {
	var content []byte
	for {
		var available uint32
		if ok, _, _ := procWinHTTPQueryDataAvailable.Call(
			request, uintptr(unsafe.Pointer(&available)),
		); ok == 0 {
			return nil, errors.New("lecture interrompue")
		}
		if available == 0 {
			return content, nil
		}
		if len(content)+int(available) > maxDownloadBytes {
			return nil, errors.New("reponse trop volumineuse")
		}

		buffer := make([]byte, available)
		var read uint32
		if ok, _, _ := procWinHTTPReadData.Call(
			request, uintptr(unsafe.Pointer(&buffer[0])), uintptr(available),
			uintptr(unsafe.Pointer(&read)),
		); ok == 0 {
			return nil, errors.New("lecture interrompue")
		}
		content = append(content, buffer[:read]...)
	}
}

// splitHTTPS separe une adresse en machine et chemin. Seul https est accepte :
// une mise a jour ne doit pas voyager en clair.
func splitHTTPS(address string) (host, path string, err error) {
	rest, found := strings.CutPrefix(address, "https://")
	if !found {
		return "", "", errors.New("adresse non securisee refusee : " + address)
	}
	host, path, found = strings.Cut(rest, "/")
	if !found {
		return host, "/", nil
	}
	return host, "/" + path, nil
}
