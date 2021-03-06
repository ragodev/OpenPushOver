/*
	OpenClient API Spec: pushover.net/api/client
	Message API Spec: pushover.net/api
	Reference Implementation: github.com/Mechazawa/Pushover-client-protocol
	Reference Implementation: github.com/nbrownus/pushover-desktop-client
	Reference Implementation: github.com/AlekSi/pushover

	TODO:
		- Improve encryption with public/private keys
		- Account for errors returned by the API
		- Fix message priority not being parsed by fetchmessages
		- Add disable device
*/

package pushover

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"
)

const (
	BaseUrl   = "https://api.pushover.net/1"
	ClientUrl = "https://client.pushover.net"
)

// Message priority
const (
	LowestPriority  = -2 // lowest priority, no notification
	LowPriority     = -1 // low priority, no sound and vibration
	NormalPriority  = 0  // normal priority, default
	HighPriority    = 1  // high priority, always with sound and vibration
	HighestPriority = 2  // emergency priority, requires acknowledge
)

// Message sound
// Should be used for speciying sounds before sending messages
const (
	PushoverSound     = "pushover"
	BikeSound         = "bike"
	BugleSound        = "bugle"
	CashregisterSound = "cashregister"
	ClassicalSound    = "classical"
	CosmicSound       = "cosmic"
	FallingSound      = "falling"
	GamelanSound      = "gamelan"
	IncomingSound     = "incoming"
	IntermissionSound = "intermission"
	MagicSound        = "magic"
	MechanicalSound   = "mechanical"
	PianobarSound     = "pianobar"
	SirenSound        = "siren"
	SpacealarmSound   = "spacealarm"
	TugboatSound      = "tugboat"
	AlienSound        = "alien"
	ClimbSound        = "climb"
	PersistentSound   = "persistent"
	EchoSound         = "echo"
	UpdownSound       = "updown"
	NoneSound         = "none"
)

// Errors
var (
	ErrNotLicensed    = errors.New("Device is not licensed")
	ErrFetchSound     = errors.New("Unable to fetch sound file")
	ErrFetchInvalid   = errors.New("Invalid sound name specified")
	ErrFetchImage     = errors.New("Unable to fetch image")
	ErrLoginFailed    = errors.New("Failed to login")
	ErrDeviceRegister = errors.New("Device register failed")
	ErrPullMsg        = errors.New("Unable to fetch new messages")
	ErrMarkRead       = errors.New("Markread messages failed")
	ErrPushMsg        = errors.New("Unable to push message")
	ErrReceipt        = errors.New("Unable to get receipt")
	ErrDeviceAuth     = errors.New("Device not authenticated")
	ErrUserPassword   = errors.New("User Password not specified")
	ErrUserName       = errors.New("UserName not specified")
	ErrMessageLimit   = errors.New("Message is longer than the limit")
)

// Push Response error structure
type PushRespErr struct {
	Query string // Query made to the API
	Err   error  // Error message
}

func (e *PushRespErr) Error() string {

	return e.Query + ": " + e.Err.Error()
}

type Client struct {
	Dial func(network, addr string) (net.Conn, error)

	UserName     string // Username
	UserPassword string // User password

	DeviceName string // Device name
	DeviceUUID string // Device UUID
	deviceOS   string // Device OS. Should only be single chars such as A (Android), F (Firefox), C (Chrome), or O (Open Client)

	Key string // Key to use for message encryption and decryption

	Login            Login
	RegisterResponse RegisterResponse
	MessagesResponse MessagesResponse
	MarkReadResponse MarkReadResponse

	AppToken string // Application Token
	UserKey  string // User Key

	Accounting struct {
		AppLimit     string
		AppRemaining string
		AppReset     string
	}

	PushResponse PushResponse
}

func (c *Client) dial(network, addr string) (net.Conn, error) {

	if c.Dial != nil {

		return c.Dial(network, addr)
	}

	dialer := &net.Dialer{

		DualStack: true,
	}

	return dialer.Dial(network, addr)
}

// Pass the sound name to fetch the apropiate sound
func (c *Client) FetchSound(sound string) (body []byte, err error) {

	urlF := fmt.Sprintf("%s/sounds/%s.wav", ClientUrl, sound)
	httpClient := &http.Client{Transport: &http.Transport{Dial: c.dial}}
	resp, err := httpClient.Get(urlF)
	if err != nil {

		err = &PushRespErr{Query: urlF, Err: err}
		return
	}
	if resp.StatusCode >= 400 {

		err = &PushRespErr{Query: urlF, Err: ErrFetchSound}
		return
	}
	defer resp.Body.Close()

	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {

		err = &PushRespErr{Query: urlF, Err: err}
		return
	}

	return
}

// Pass the icon id to fetch the apropiate image
func (c *Client) FetchImage(icon string) (body []byte, err error) {

	urlF := fmt.Sprintf("%s/icons/%s.png", ClientUrl, icon)
	httpClient := &http.Client{Transport: &http.Transport{Dial: c.dial}}
	resp, err := httpClient.Get(urlF)
	if err != nil {

		err = &PushRespErr{Query: urlF, Err: err}
		return
	}
	if resp.StatusCode >= 400 {

		err = &PushRespErr{Query: urlF, Err: ErrFetchImage}
		return
	}
	defer resp.Body.Close()

	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {

		err = &PushRespErr{Query: urlF, Err: err}
		return
	}

	return
}

type Login struct {
	Status  int    `json: "status"`
	Secret  string `json: "secret"`
	Request string `json: "request"`
	ID      string `json: "id"`
}

func (c *Client) LoginDevice() (err error) {

	// Some validiation
	if len(c.UserName) < 1 {

		return ErrUserName
	}
	if len(c.UserPassword) < 1 {

		return ErrUserPassword
	}

	err = VerifyDeviceName(c.DeviceName)
	if err != nil {

		return
	}

	// Set the unexported feilds
	c.deviceOS = "O"

	vars := url.Values{}
	vars.Add("email", c.UserName)
	vars.Add("password", c.UserPassword)

	urlF := fmt.Sprintf("%s%s", BaseUrl, "/users/login.json")
	httpClient := &http.Client{Transport: &http.Transport{Dial: c.dial}}
	resp, err := httpClient.PostForm(urlF, vars)
	if err != nil {

		return &PushRespErr{Query: urlF, Err: err}
	}
	if resp.StatusCode >= 400 {

		return &PushRespErr{Query: urlF, Err: ErrLoginFailed}
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {

		return &PushRespErr{Query: urlF, Err: err}
	}

	err = json.Unmarshal(body, &c.Login)
	if err != nil {

		return &PushRespErr{Query: urlF, Err: err}
	}

	return
}

type RegisterResponse struct {
	Status  int    `json: "status"`
	Request string `json: "request"`
	ID      string `json: "id"`
}

func (c *Client) RegisterDevice() (err error) {

	if len(c.Login.Secret) < 1 {

		return ErrDeviceAuth
	}

	err = VerifyDeviceName(c.DeviceName)
	if err != nil {

		return
	}

	vars := url.Values{}
	vars.Add("secret", c.Login.Secret)
	vars.Add("name", c.DeviceName)
	vars.Add("os", c.deviceOS)

	urlF := fmt.Sprintf("%s%s", BaseUrl, "/devices.json")
	httpClient := &http.Client{Transport: &http.Transport{Dial: c.dial}}
	resp, err := httpClient.PostForm(urlF, vars)
	if err != nil {

		return &PushRespErr{Query: urlF, Err: err}
	}
	if resp.StatusCode >= 400 {

		return &PushRespErr{Query: urlF, Err: ErrDeviceRegister}
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {

		return &PushRespErr{Query: urlF, Err: err}
	}

	err = json.Unmarshal(body, &c.RegisterResponse)
	if err != nil {

		return &PushRespErr{Query: urlF, Err: err}
	}
	c.DeviceUUID = c.RegisterResponse.ID

	return
}

type MessagesResponse struct {
	Messages []PullMessage `json: "messages"`
	User     User          `json: "user"`

	Status  int    `json: "status"`
	Request string `json: "request"`
}

// Pull Message structure
type PullMessage struct {
	ID       int    `json: "id"`
	Umid     int    `json: "umid"`
	Title    string `json: "title"`
	Message  string `json: "message"`
	App      string `json: "app"`
	Aid      int    `json: "aid"`
	Icon     string `json: "icon"`
	Date     int64  `json: "date"`
	Priority int    `json: "priority"`
	Sound    string `json: "sound"`
	Url      string `json: "url"`
	UrlTitle string `json: "url_title"`
	Acked    int    `json: "acked"`
	Receipt  int    `json: "receipt"`
}

type User struct {
	QuietHours        bool `json: "quiet_hours"`
	IsAndroidLicensed bool `json: "is_android_licensed"` // Was the app bought on android?
	IsIOSLicensed     bool `json: "is_ios_licensed"`     // Was the app bought on IOS
	IsDesktopLicensed bool `json: "is_desktop_licensed"` // Was the app bought on the Pushover store
}

func (c *Client) FetchMessages() (fetched int, err error) {

	if len(c.Login.Secret) < 1 {

		err = ErrDeviceAuth
		return
	}

	vars := url.Values{}
	vars.Add("secret", c.Login.Secret)
	vars.Add("device_id", c.DeviceUUID)

	urlF := fmt.Sprintf("%s%s%s", BaseUrl, "/messages.json?", vars.Encode())
	httpClient := &http.Client{Transport: &http.Transport{Dial: c.dial}}
	resp, err := httpClient.Get(urlF)
	if err != nil {

		err = &PushRespErr{Query: urlF, Err: err}
		return
	}
	if resp.StatusCode >= 400 {

		err = &PushRespErr{Query: urlF, Err: ErrPullMsg}
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {

		err = &PushRespErr{Query: urlF, Err: err}
		return
	}

	err = json.Unmarshal(body, &c.MessagesResponse)
	if err != nil {

		err = &PushRespErr{Query: urlF, Err: err}
		return
	}
	fetched = len(c.MessagesResponse.Messages)

	for _, v := range c.MessagesResponse.Messages {

		// Decrypt the message if required
		if len(c.Key) > 0 && isEncrypted(v.Message) {

			msg, err := decryptMessage(c.Key, v.Message)
			if err != nil {

				return 0, err
			}
			v.Message = msg
		}

		// According to the API spec the title should contain the application name if empty
		if len(v.Title) < 1 {

			v.Title = v.App
		}

		// Make sure an icon is specified in the message
		if len(v.Icon) < 1 {

			v.Icon = "default"
		}
	}

	return
}

type MarkReadResponse struct {
	Status  int    `json: "status"`
	Request string `json: "request"`
}

func (c *Client) MarkReadHighest() (err error) {

	return c.MarkRead(c.MessagesResponse.Messages[len(c.MessagesResponse.Messages)-1].ID)
}

func (c *Client) MarkRead(id int) (err error) {

	if len(c.Login.Secret) < 1 {

		return ErrDeviceAuth
	}

	vars := url.Values{}
	vars.Add("secret", c.Login.Secret)
	vars.Add("message", strconv.Itoa(id))

	urlF := fmt.Sprintf("%s%s%s%s", BaseUrl, "/devices/", c.DeviceUUID, "/update_highest_message.json")
	httpClient := &http.Client{Transport: &http.Transport{Dial: c.dial}}
	resp, err := httpClient.PostForm(urlF, vars)
	if err != nil {

		return &PushRespErr{Query: urlF, Err: err}
	}
	if resp.StatusCode >= 400 {

		return &PushRespErr{Query: urlF, Err: ErrMarkRead}
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {

		return &PushRespErr{Query: urlF, Err: err}
	}

	err = json.Unmarshal(body, &c.MarkReadResponse)
	if err != nil {

		return &PushRespErr{Query: urlF, Err: err}
	}

	return
}

type PushResponse struct {
	Receipt string `json: "receipt"`

	Status  int      `json: "status"`
	Request string   `json: "request"`
	Errors  []string `json: "errors"`
}

func (c *Client) Push(message string) (err error) {

	msg := PushMessage{

		Message: message,
	}
	return c.PushMessage(msg, false)
}

// Push message structure
type PushMessage struct {
	Device    string // Device name to send to specific devices instead of all
	Title     string // Title of message
	Message   string // Message to send
	Priority  int    // Defaults to 0 although expire needs to be specified when higher than 1
	Url       string // Url to be sent with message
	UrlTitle  string // Url title
	Timestamp int64  // Timestamp which should be a unixstamp
	Sound     string // Sound to be played on client device

	// Emergency notifications
	Expire   int
	Retry    int
	Callback string
}

func (c *Client) PushMessage(msg PushMessage, encrypt bool) (err error) {

	// Some validations
	err = VerifyUserKey(c.AppToken)
	if err != nil {

		return
	}

	err = VerifyUserKey(c.UserKey)
	if err != nil {

		return
	}

	err = VerifyPushMessage(msg)
	if err != nil {

		return
	}

	if encrypt {

		err := c.encryptMessage(msg)
		if err != nil {

			return err
		}
	}

	vars := url.Values{}
	// Required
	vars.Add("token", c.AppToken)
	vars.Add("user", c.UserKey)
	vars.Add("message", msg.Message)
	// Optional
	vars.Add("device", msg.Device)
	vars.Add("title", msg.Title)
	vars.Add("url", msg.Url)
	vars.Add("url_title", msg.UrlTitle)
	vars.Add("expire", string(msg.Expire))
	vars.Add("retry", string(msg.Retry))
	vars.Add("priority", string(msg.Priority))
	vars.Add("timestamp", strconv.FormatInt(msg.Timestamp, 10))
	vars.Add("sound", msg.Sound)
	vars.Add("callback", msg.Callback)

	urlF := fmt.Sprintf("%s%s", BaseUrl, "/messages.json")
	httpClient := &http.Client{Transport: &http.Transport{Dial: c.dial}}
	resp, err := httpClient.PostForm(urlF, vars)
	if err != nil {

		return &PushRespErr{Query: urlF, Err: err}
	}
	if resp.StatusCode >= 400 {

		return &PushRespErr{Query: urlF, Err: ErrPushMsg}
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {

		return &PushRespErr{Query: urlF, Err: err}
	}

	err = json.Unmarshal(body, &c.PushResponse)
	if err != nil {

		return &PushRespErr{Query: urlF, Err: err}
	}

	// Update accounting
	c.Accounting.AppLimit = resp.Header.Get("X-Limit-App-Limit")
	c.Accounting.AppRemaining = resp.Header.Get("X-Limit-App-Remaining")
	c.Accounting.AppReset = resp.Header.Get("X-Limit-App-Reset")

	return
}

type Receipt struct {
	Acknowledged    int   `json: "acknowledged"`
	AcknowledgedAt  int   `json: "acknowledged_at"`
	LastDeliveredAt int   `json: "last_delivered_at"`
	Expired         int   `json: "expired"`
	ExpiresAt       int64 `json: "expires_at"`
	CalledBack      int   `json: "called_back"`
	CalledBackAt    int64 `json: "called_back_at"`

	Status  int    `json: "status"`
	Request string `json: "request"`
}

func (c *Client) GetReceipt(receipt string) (err error) {

	err = VerifyReceipt(receipt)
	if err != nil {

		return
	}

	urlF := fmt.Sprintf("%s/%s.json?token=%s", BaseUrl, "/receipts", receipt, c.AppToken)
	httpClient := &http.Client{Transport: &http.Transport{Dial: c.dial}}
	resp, err := httpClient.Get(urlF)
	if err != nil {

		return &PushRespErr{Query: urlF, Err: err}
	}
	if resp.StatusCode >= 400 {

		return &PushRespErr{Query: urlF, Err: ErrReceipt}
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {

		return &PushRespErr{Query: urlF, Err: err}
	}

	err = json.Unmarshal(body, &c.PushResponse)
	if err != nil {

		return &PushRespErr{Query: urlF, Err: err}
	}

	return
}
