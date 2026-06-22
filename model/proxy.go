package model

import (
	"strconv"
	"time"
)

// Proxy represents an outbound proxy for channel requests.
// Ported from sub2api's proxy schema.
type Proxy struct {
	Id        int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Name      string    `json:"name" gorm:"type:varchar(100);not null;uniqueIndex"`
	Protocol  string    `json:"protocol" gorm:"type:varchar(20);default:'http'"` // http, https, socks5
	Host      string    `json:"host" gorm:"type:varchar(255);not null"`
	Port      int       `json:"port" gorm:"not null"`
	Username  string    `json:"username" gorm:"type:varchar(255);default:''"`
	Password  string    `json:"password" gorm:"type:varchar(255);default:''"`
	Status    string    `json:"status" gorm:"type:varchar(20);default:'active'"` // active, inactive
	Note      string    `json:"note" gorm:"type:text"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Proxy) TableName() string {
	return "proxies"
}

// ProxyURL returns the full proxy URL string
func (p *Proxy) ProxyURL() string {
	if p.Username != "" {
		return p.Protocol + "://" + p.Username + ":" + p.Password + "@" + p.Host + ":" + strconv.Itoa(p.Port)
	}
	return p.Protocol + "://" + p.Host + ":" + strconv.Itoa(p.Port)
}



// --- CRUD ---

func ListProxies() ([]Proxy, error) {
	var proxies []Proxy
	err := DB.Order("id ASC").Find(&proxies).Error
	return proxies, err
}

func GetProxy(id int64) (*Proxy, error) {
	var proxy Proxy
	err := DB.First(&proxy, id).Error
	return &proxy, err
}

func CreateProxy(proxy *Proxy) error {
	return DB.Create(proxy).Error
}

func UpdateProxy(proxy *Proxy) error {
	return DB.Save(proxy).Error
}

func DeleteProxy(id int64) error {
	return DB.Delete(&Proxy{}, id).Error
}

func ListActiveProxies() ([]Proxy, error) {
	var proxies []Proxy
	err := DB.Where("status = ?", "active").Order("id ASC").Find(&proxies).Error
	return proxies, err
}
