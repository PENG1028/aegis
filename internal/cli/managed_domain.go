package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"aegis/internal/manageddomain"
	"aegis/internal/service"

	"github.com/spf13/cobra"
)

func newManagedDomainCommand(mdSvc *manageddomain.AppService, svcSvc *service.AppService) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "managed-domain",
		Short: "Manage managed domains",
		Long:  "Add, verify, enable, disable, and list managed domains.",
	}

	cmd.AddCommand(newMDAddCommand(mdSvc, svcSvc))
	cmd.AddCommand(newMDVerifyCommand(mdSvc))
	cmd.AddCommand(newMDEnableCommand(mdSvc))
	cmd.AddCommand(newMDDisableCommand(mdSvc))
	cmd.AddCommand(newMDListCommand(mdSvc))

	return cmd
}

func newMDAddCommand(mdSvc *manageddomain.AppService, svcSvc *service.AppService) *cobra.Command {
	var serviceName, owner, targetType, targetRef string

	cmd := &cobra.Command{
		Use:   "add <domain>",
		Short: "Add a managed domain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if serviceName == "" {
				return fmt.Errorf("--service is required")
			}
			if owner == "" {
				return fmt.Errorf("--owner is required")
			}

			ctx := context.Background()
			svc, err := svcSvc.GetService(ctx, serviceName)
			if err != nil {
				return err
			}

			md, err := mdSvc.CreateManagedDomain(ctx, manageddomain.CreateManagedDomainInput{
				Domain:     args[0],
				ServiceID:  svc.ID,
				OwnerRef:   owner,
				TargetType: targetType,
				TargetRef:  targetRef,
			})
			if err != nil {
				return err
			}

			fmt.Printf("Managed domain %q created (ID: %s)\n", md.Domain, md.ID)
			fmt.Println()
			fmt.Println("DNS Verification Required:")
			fmt.Printf("  Type:  %s\n", md.VerificationType)
			fmt.Printf("  Name:  %s\n", md.VerificationName)
			fmt.Printf("  Value: %s\n", md.VerificationValue)
			fmt.Println()
			fmt.Println("Create a TXT record with the above values, then run:")
			fmt.Printf("  aegis managed-domain verify %s\n", md.Domain)
			return nil
		},
	}

	cmd.Flags().StringVar(&serviceName, "service", "", "Service name or ID (required)")
	cmd.Flags().StringVar(&owner, "owner", "", "Owner reference (required)")
	cmd.Flags().StringVar(&targetType, "target-type", "", "Target type: auth_page, service_page, hosting, custom")
	cmd.Flags().StringVar(&targetRef, "target-ref", "", "Target reference")
	return cmd
}

func newMDVerifyCommand(mdSvc *manageddomain.AppService) *cobra.Command {
	return &cobra.Command{
		Use:   "verify <domain-or-id>",
		Short: "Verify a managed domain via DNS",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			md, err := mdSvc.VerifyDomain(ctx, args[0])
			if err != nil {
				return err
			}
			fmt.Printf("Domain %q verification: %s\n", md.Domain, md.Status)
			fmt.Printf("  Message: %s\n", md.LastCheckMessage)
			if md.Status == "verified" {
				fmt.Println()
				fmt.Printf("Run 'aegis managed-domain enable %s' to activate.\n", md.Domain)
			}
			return nil
		},
	}
}

func newMDEnableCommand(mdSvc *manageddomain.AppService) *cobra.Command {
	return &cobra.Command{
		Use:   "enable <domain-or-id>",
		Short: "Enable a verified managed domain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			md, err := mdSvc.EnableDomain(ctx, args[0])
			if err != nil {
				return err
			}
			fmt.Printf("Managed domain %q enabled (status: %s)\n", md.Domain, md.Status)
			fmt.Println("Run 'aegis apply' to generate the Caddy configuration.")
			return nil
		},
	}
}

func newMDDisableCommand(mdSvc *manageddomain.AppService) *cobra.Command {
	return &cobra.Command{
		Use:   "disable <domain-or-id>",
		Short: "Disable a managed domain",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			md, err := mdSvc.DisableDomain(ctx, args[0])
			if err != nil {
				return err
			}
			fmt.Printf("Managed domain %q disabled.\n", md.Domain)
			return nil
		},
	}
}

func newMDListCommand(mdSvc *manageddomain.AppService) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all managed domains",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			domains, err := mdSvc.ListManagedDomains(ctx)
			if err != nil {
				return err
			}

			if len(domains) == 0 {
				fmt.Println("No managed domains.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "DOMAIN\tSERVICE\tOWNER\tSTATUS\tTLS")
			for _, md := range domains {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					md.Domain, md.ServiceID, md.OwnerRef, md.Status, md.TLSStatus)
			}
			w.Flush()
			return nil
		},
	}
}
