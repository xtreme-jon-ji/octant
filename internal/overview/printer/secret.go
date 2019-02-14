package printer

import (
	"fmt"

	"github.com/heptio/developer-dash/internal/overview/link"

	"github.com/heptio/developer-dash/internal/view/component"
	"github.com/heptio/developer-dash/internal/view/flexlayout"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
)

var (
	secretTableCols = component.NewTableCols("Name", "Labels", "Type", "Data", "Age")
	secretDataCols  = component.NewTableCols("Key")
)

// SecretListHandler is a printFunc that lists secrets.
func SecretListHandler(list *corev1.SecretList, opts Options) (component.ViewComponent, error) {
	if list == nil {
		return nil, errors.New("list of secrets is nil")
	}

	table := component.NewTable("Secrets", secretTableCols)

	for _, secret := range list.Items {
		row := component.TableRow{}

		row["Name"] = link.ForObject(&secret, secret.Name)
		row["Labels"] = component.NewLabels(secret.ObjectMeta.Labels)
		row["Type"] = component.NewText(string(secret.Type))
		row["Data"] = component.NewText(fmt.Sprintf("%d", len(secret.Data)))
		row["Age"] = component.NewTimestamp(secret.ObjectMeta.CreationTimestamp.Time)

		table.Add(row)
	}

	return table, nil
}

// SecretHandler is a printFunc for printing a secret summary.
func SecretHandler(secret *corev1.Secret, options Options) (component.ViewComponent, error) {
	if secret == nil {
		return nil, errors.New("secret is nil")
	}
	fl := flexlayout.New()

	configSection := fl.AddSection()
	configView, err := secretConfiguration(*secret)
	if err != nil {
		return nil, errors.Wrapf(err, "summarize configuration for secret %s", secret.Name)
	}
	if err := configSection.Add(configView, 16); err != nil {
		return nil, errors.Wrap(err, "add secret config to layout")
	}

	metadata, err := NewMetadata(secret)
	if err != nil {
		return nil, errors.Wrap(err, "create metadata generator")
	}

	if err := metadata.AddToFlexLayout(fl); err != nil {
		return nil, errors.Wrap(err, "add metadata to layout")
	}

	dataSection := fl.AddSection()
	dataView, err := secretData(*secret)
	if err != nil {
		return nil, errors.Wrapf(err, "summary data for secret %s", secret.Name)
	}
	if err := dataSection.Add(dataView, 24); err != nil {
		return nil, errors.Wrap(err, "add secret data to layout")
	}

	return fl.ToComponent("Summary"), nil
}

func secretConfiguration(secret corev1.Secret) (*component.Summary, error) {
	var sections []component.SummarySection

	sections = append(sections, component.SummarySection{
		Header:  "Type",
		Content: component.NewText(string(secret.Type)),
	})

	summary := component.NewSummary("Configuration", sections...)
	return summary, nil
}

func secretData(secret corev1.Secret) (*component.Table, error) {
	table := component.NewTable("Data", secretDataCols)

	for key := range secret.Data {
		row := component.TableRow{}
		row["Key"] = component.NewText(key)

		table.Add(row)
	}

	return table, nil
}
