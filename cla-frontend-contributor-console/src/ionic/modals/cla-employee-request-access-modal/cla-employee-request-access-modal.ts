// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

import { ChangeDetectorRef, Component } from '@angular/core';
import { AlertController, IonicPage, ModalController, NavController, NavParams, ViewController } from 'ionic-angular';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { EmailValidator } from '../../validators/email';
import { ClaService } from '../../services/cla.service';

@IonicPage({
  segment: 'cla/project/:projectId/repository/:repositoryId/user/:userId/employee/company/contact'
})
@Component({
  selector: 'cla-employee-request-access-modal',
  templateUrl: 'cla-employee-request-access-modal.html'
})
export class ClaEmployeeRequestAccessModal {
  project: any;
  projectId: string;
  repositoryId: string;
  userId: string;
  companyId: string;
  company: any;
  authenticated: boolean;
  cclaSignature: any;
  managers: any;
  formErrors: any[];

  userEmails: Array<string>;

  form: FormGroup;
  submitAttempt: boolean = false;
  currentlySubmitting: boolean = false;
  loading: any;
  showManagerSelectOption: boolean;
  showManagerEnterOption: boolean;

  constructor(
    public navCtrl: NavController,
    public navParams: NavParams,
    public modalCtrl: ModalController,
    public viewCtrl: ViewController,
    public alertCtrl: AlertController,
    private changeDetectorRef: ChangeDetectorRef,
    private formBuilder: FormBuilder,
    private claService: ClaService
  ) {
    this.getDefaults();
    this.loading = true;
    this.project = {};
    this.company = {};

    this.projectId = navParams.get('projectId');
    this.repositoryId = navParams.get('repositoryId');
    this.userId = navParams.get('userId');
    this.companyId = navParams.get('companyId');
    this.authenticated = navParams.get('authenticated');
    this.form = formBuilder.group({
      user_email: ['', Validators.compose([Validators.required, EmailValidator.isValid])],
      message: [''],
      recipient_name: [''],
      recipient_email: [''],
      manager: [''],
      managerOptions: ['', Validators.compose([Validators.required])]
    });
    this.managers = [];
    this.formErrors = [];
  }

  saveManagerOption() {
    const option = this.form.value.managerOptions;
    if (option === 'select manager') {
      this.showManagerSelectOption = true;
      this.showManagerEnterOption = false;
      this.resetFormValues('recipient_name');
      this.resetFormValues('recipient_email');
    } else if (option === 'enter manager') {
      this.showManagerSelectOption = false;
      this.showManagerEnterOption = true;
      this.resetFormValues('manager');
    }
  }

  resetFormValues(value) {
    return this.form.controls[value].reset();
  }

  getCLAManagerDetails(managerId) {
    const manager = this.managers.filter((manager) => {
      return (manager.userID = managerId);
    });
    return manager;
  }

  getDefaults() {
    this.userEmails = [];
  }

  ngOnInit() {
    this.getUser(this.userId, this.authenticated).subscribe((user) => {
      if (user) {
        this.userEmails = user.user_emails || [];
        if (user.lf_email && this.userEmails.indexOf(user.lf_email) == -1) {
          this.userEmails.push(user.lf_email);
        }
      }
    });
    this.getProject(this.projectId);
    this.getCompany(this.companyId);
    this.getProjectSignatures(this.projectId, this.companyId);
  }

  getUser(userId: string, authenticated: boolean) {
    if (authenticated) {
      // Gerrit Users
      return this.claService.getUserWithAuthToken(userId);
    } else {
      // Github Users
      return this.claService.getUser(userId);
    }
  }

  getProject(projectId: string) {
    this.claService.getProject(projectId).subscribe((response) => {
      this.project = response;
    });
  }

  getCompany(companyId: string) {
    this.claService.getCompany(companyId).subscribe((response) => {
      this.company = response;
    });
  }

  insertAndSortManagersList(manager) {
    this.managers.push(manager);
    this.managers.sort((first, second) => {
      return first.username.toLowerCase() - second.username.toLowerCase();
    });
  }

  getProjectSignatures(projectId: string, companyId: string) {
    // Get CCLA Company Signatures - should just be one
    this.loading = true;
    this.claService.getCompanyProjectSignatures(companyId, projectId).subscribe(
      (response) => {
        this.loading = false;
        console.log('Signatures for project: ' + projectId + ' for company: ' + companyId);
        console.log(response);
        if (response.signatures) {
          let cclaSignatures = response.signatures.filter((sig) => sig.signatureType === 'ccla');
          console.log('CCLA Signatures for project: ' + cclaSignatures.length);
          if (cclaSignatures.length) {
            console.log('CCLA Signatures for project id: ' + projectId + ' and company id: ' + companyId);
            console.log(cclaSignatures);
            this.cclaSignature = cclaSignatures[0];
            console.log(this.cclaSignature);
            console.log(this.cclaSignature.signatureACL);
            if (this.cclaSignature.signatureACL != null) {
              for (let manager of this.cclaSignature.signatureACL) {
                this.insertAndSortManagersList({
                  userID: manager.userID,
                  username: manager.username,
                  lfEmail: manager.lfEmail
                });
              }
            }
          }
        }
      },
      (exception) => {
        this.loading = false;
        console.log(
          'Exception while calling: getCompanyProjectSignatures() for company ID: ' +
            companyId +
            ' and project ID: ' +
            projectId
        );
        console.log(exception);
      }
    );
  }

  // ContactUpdateModal modal dismiss
  dismiss() {
    this.viewCtrl.dismiss();
  }

  submit() {
    this.submitAttempt = true;
    this.currentlySubmitting = true;
    this.formErrors = [];
    console.log("Form");
    console.log(this.form);
    let data = {
      company_id: this.companyId,
      user_id: this.userId,
      user_email: this.form.value.user_email,
      project_id: this.projectId,
      message: this.form.value.message,
      recipient_name:
        this.form.value.recipient_name || this.form.value.manager
          ? this.getCLAManagerDetails(this.form.value.message)[0].username
          : undefined,
      recipient_email:
        this.form.value.recipient_email || this.form.value.manager
          ? this.getCLAManagerDetails(this.form.value.message)[0].lfEmail
          : undefined
    };

    if (!this.form.valid) {
      this.getFormValidationErrors();
      this.currentlySubmitting = false;
      // prevent submit
      return;
    }
    this.claService.postUserMessageToCompanyManager(this.userId, this.companyId, data).subscribe((response) => {
      this.loading = true;
      this.emailSent();
    });

  }

  saveWhiteListRequest() {
    let user = {
      userId: this.userId
    }
    this.claService.postCCLAWhitelistRequest(this.companyId, this.projectId, user).subscribe(
      () => {
        console.log(this.userId+ ' ccla whitelist request for project: ' + this.projectId + ' for company: ' + this.companyId);
      },
      (exception) => {
        console.log('Exception during ccla whitelist request for user ' + this.userId + ' on project: ' + this.projectId + ' and company: ' + this.companyId);
        console.log(exception);
      }
    );
  }

  emailSent() {
    this.loading = false;
    this.saveWhiteListRequest();
    let message = this.authenticated
      ? "Thank you for contacting your company's administrators. Once the CLA is signed and you are authorized, please navigate to the Agreements tab in the Gerrit Settings page and restart the CLA signing process"
      : "Thank you for contacting your company's administrators. Once the CLA is signed and you are authorized, you will have to complete the CLA process from your existing pull request.";
    let alert = this.alertCtrl.create({
      title: 'E-Mail Successfully Sent!',
      subTitle: message,
      buttons: ['Dismiss']
    });
    alert.onDidDismiss(() => this.dismiss());
    alert.present();
  }

  getFormValidationErrors() {
    let message;
    Object.keys(this.form.controls).forEach((key) => {
      const controlErrors = this.form.get(key).errors;
      if (controlErrors != null) {
        Object.keys(controlErrors).forEach((keyError) => {
          switch (key) {
            case 'managerOptions':
              message = `*Selecting an Option for Entering a CLA Manager is ${keyError}`;
              break;
            case 'user_email':
              message = `*Email Authorize Field is ${keyError}`;
              break;
            default:
              message = `Check Fields for errors`;
          }
          this.formErrors.push({
            message
          });
        });
      }
    });
  }
}
